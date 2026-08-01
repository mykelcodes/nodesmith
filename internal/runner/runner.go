package runner

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"nodesmith/internal/environ"
	"nodesmith/internal/planner"
)

const (
	defaultRingCapacity       = 10_000
	defaultEventQueueCapacity = 10_000
	defaultRetention          = 30 * time.Minute
	// orphanedOutputGrace bounds how long a step keeps reading output after its
	// own process has exited. A surviving grandchild can hold the inherited
	// write ends open indefinitely; without this bound the step would never
	// reach a terminal state and the process tree would be orphaned at shutdown.
	orphanedOutputGrace = 2 * time.Second
	// processWaitDelay bounds how long a killed process may ignore termination
	// before the runtime force-closes its descriptors.
	processWaitDelay = 2 * time.Second
)

// State is the lifecycle state of a scaffold job.
type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateSuccess   State = "success"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// Job is a stable snapshot of a scaffold execution.
type Job struct {
	ID         string    `json:"id"`
	State      State     `json:"state"`
	StepIndex  int       `json:"stepIndex"`
	StepCount  int       `json:"stepCount"`
	ExitCode   int       `json:"exitCode"`
	ProjectDir string    `json:"projectDir"`
	StartedAt  time.Time `json:"startedAt"`
	EndedAt    time.Time `json:"endedAt"`
	Error      string    `json:"error"`
}

// LogLine is one line captured from a process stream.
type LogLine struct {
	Seq    int    `json:"seq"`
	Stream string `json:"stream"`
	Text   string `json:"text"`
	StepID string `json:"stepId"`
}

// StartedEvent is emitted once a job begins.
type StartedEvent struct {
	JobID      string `json:"jobId"`
	ProjectDir string `json:"projectDir"`
	StepCount  int    `json:"stepCount"`
}

// StepEvent describes one step state transition.
type StepEvent struct {
	JobID  string `json:"jobId"`
	StepID string `json:"stepId"`
	Index  int    `json:"index"`
	Total  int    `json:"total"`
	State  string `json:"state"`
}

// LogEvent adds a job id to a captured log line.
type LogEvent struct {
	JobID  string `json:"jobId"`
	Seq    int    `json:"seq"`
	StepID string `json:"stepId"`
	Stream string `json:"stream"`
	Text   string `json:"text"`
}

// DoneEvent is emitted once with a terminal job state.
type DoneEvent struct {
	JobID      string `json:"jobId"`
	State      State  `json:"state"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
	ProjectDir string `json:"projectDir"`
	Error      string `json:"error"`
}

// Event carries a namespaced event name and a typed payload.
type Event struct {
	Name    string
	Payload any
}

type subscriber func(Event)

type logRing struct {
	buf   []LogLine
	start int
	size  int
}

func newLogRing(capacity int) logRing {
	return logRing{buf: make([]LogLine, capacity)}
}

func (r *logRing) append(line LogLine) {
	if len(r.buf) == 0 {
		return
	}
	if r.size < len(r.buf) {
		r.buf[(r.start+r.size)%len(r.buf)] = line
		r.size++
		return
	}
	r.buf[r.start] = line
	r.start = (r.start + 1) % len(r.buf)
}

// after returns the retained lines with a sequence number at or above seq.
//
// Sequence numbers are assigned monotonically and the ring holds a contiguous
// run of them, so the first matching offset is arithmetic rather than a scan,
// and the result is sized to what is actually returned. This is on the log
// polling path, which the run page hits repeatedly while a job is live.
//
// The result is always non-nil: it crosses the Wails bridge as JSON, where a
// nil slice would arrive as null rather than an empty array.
func (r *logRing) after(seq int) []LogLine {
	if r.size == 0 {
		return []LogLine{}
	}
	offset := seq - r.buf[r.start].Seq
	if offset < 0 {
		offset = 0
	}
	if offset >= r.size {
		return []LogLine{}
	}
	result := make([]LogLine, 0, r.size-offset)
	for i := offset; i < r.size; i++ {
		result = append(result, r.buf[(r.start+i)%len(r.buf)])
	}
	return result
}

type jobRecord struct {
	job    Job
	plan   planner.Plan
	cancel context.CancelFunc

	// logMu guards logs and nextSeq independently of Manager.mu so that a
	// chatty generator does not serialise Status and Logs callers behind every
	// captured line. It is also what keeps sequence assignment, ring insertion,
	// and event publication atomic, so published log events stay ordered.
	logMu   sync.Mutex
	logs    logRing
	nextSeq int
}

// Manager owns job execution, replayable logs, cancellation, and lifecycle events.
type Manager struct {
	mu           sync.RWMutex
	jobs         map[string]*jobRecord
	activeJobID  string
	ringCapacity int
	retention    time.Duration
	subscribers  map[uint64]subscriber
	nextSubID    uint64
	pathProvider func() (string, error)

	eventMu     sync.Mutex
	eventQueue  []Event
	eventSignal chan struct{}
	eventLimit  int
}

// Option customises a Manager.
type Option func(*Manager)

// WithRingCapacity overrides the per-job replay buffer capacity.
func WithRingCapacity(capacity int) Option {
	return func(manager *Manager) {
		if capacity >= 0 {
			manager.ringCapacity = capacity
		}
	}
}

// WithEventQueueCapacity bounds best-effort live delivery. Replayable logs
// remain authoritative when a stalled subscriber causes live log events to be
// dropped. The capacity must be positive.
func WithEventQueueCapacity(capacity int) Option {
	return func(manager *Manager) {
		if capacity > 0 {
			manager.eventLimit = capacity
		}
	}
}

// WithRetention overrides how long terminal jobs remain in memory.
func WithRetention(retention time.Duration) Option {
	return func(manager *Manager) {
		if retention >= 0 {
			manager.retention = retention
		}
	}
}

// WithPathProvider supplies the effective executable PATH for every process.
// It is called immediately before each step so runtime PATH overrides take
// effect without recreating the manager.
func WithPathProvider(provider func() (string, error)) Option {
	return func(manager *Manager) {
		manager.pathProvider = provider
	}
}

// NewManager constructs a single-concurrency job manager.
func NewManager(options ...Option) *Manager {
	manager := &Manager{
		jobs:         make(map[string]*jobRecord),
		ringCapacity: defaultRingCapacity,
		retention:    defaultRetention,
		subscribers:  make(map[uint64]subscriber),
		eventSignal:  make(chan struct{}, 1),
		eventLimit:   defaultEventQueueCapacity,
	}
	for _, option := range options {
		option(manager)
	}
	go manager.dispatch()
	return manager
}

// Subscribe receives best-effort live events. Logs are always recoverable with Logs.
func (m *Manager) Subscribe(callback func(Event)) func() {
	m.mu.Lock()
	id := m.nextSubID
	m.nextSubID++
	m.subscribers[id] = callback
	m.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			delete(m.subscribers, id)
			m.mu.Unlock()
		})
	}
}

// Start begins executing a resolved plan and returns immediately.
func (m *Manager) Start(plan planner.Plan) (Job, error) {
	if len(plan.Steps) == 0 {
		return Job{}, errors.New("cannot start a plan with no steps")
	}
	jobID, err := newJobID()
	if err != nil {
		return Job{}, fmt.Errorf("create job id: %w", err)
	}

	m.mu.Lock()
	m.pruneLocked(time.Now())
	if m.activeJobID != "" {
		active := m.jobs[m.activeJobID]
		if active != nil && !isTerminal(active.job.State) {
			m.mu.Unlock()
			return Job{}, fmt.Errorf("another project is already being created (job %s)", active.job.ID)
		}
		m.activeJobID = ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	record := &jobRecord{
		job: Job{
			ID:         jobID,
			State:      StatePending,
			StepIndex:  -1,
			StepCount:  len(plan.Steps),
			ExitCode:   -1,
			ProjectDir: plan.ProjectDir,
		},
		plan:   plan,
		cancel: cancel,
		logs:   newLogRing(m.ringCapacity),
	}
	m.jobs[jobID] = record
	m.activeJobID = jobID
	snapshot := record.job
	m.mu.Unlock()

	go m.execute(ctx, jobID)
	return snapshot, nil
}

// Cancel requests cancellation of a pending or running job.
func (m *Manager) Cancel(jobID string) error {
	m.mu.RLock()
	record, ok := m.jobs[jobID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("job %q was not found", jobID)
	}
	cancel := record.cancel
	terminal := isTerminal(record.job.State)
	m.mu.RUnlock()
	if !terminal {
		cancel()
	}
	return nil
}

// Shutdown cancels the active process tree, if any, and waits for the runner
// to observe its terminal state. It is intended for the desktop app shutdown
// hook so generators are not orphaned when the window closes.
func (m *Manager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.RLock()
	jobID := m.activeJobID
	record := m.jobs[jobID]
	m.mu.RUnlock()
	if jobID == "" || record == nil {
		return nil
	}
	record.cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		job, err := m.Status(jobID)
		if err == nil && isTerminal(job.State) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for active job %s to stop: %w", jobID, ctx.Err())
		case <-ticker.C:
		}
	}
}

// Status returns the latest snapshot for a job.
func (m *Manager) Status(jobID string) (Job, error) {
	m.mu.RLock()
	record, ok := m.jobs[jobID]
	if !ok {
		m.mu.RUnlock()
		return Job{}, fmt.Errorf("job %q was not found", jobID)
	}
	snapshot := record.job
	m.mu.RUnlock()
	return snapshot, nil
}

// Logs returns retained lines whose sequence is at least fromSeq.
func (m *Manager) Logs(jobID string, fromSeq int) ([]LogLine, error) {
	if fromSeq < 0 {
		fromSeq = 0
	}
	m.mu.RLock()
	record, ok := m.jobs[jobID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("job %q was not found", jobID)
	}
	record.logMu.Lock()
	lines := record.logs.after(fromSeq)
	record.logMu.Unlock()
	return lines, nil
}

func (m *Manager) execute(ctx context.Context, jobID string) {
	m.mu.Lock()
	record := m.jobs[jobID]
	record.job.State = StateRunning
	record.job.StartedAt = time.Now()
	started := record.job.StartedAt
	plan := record.plan
	m.mu.Unlock()

	m.publish(Event{
		Name: "nodesmith:job:started",
		Payload: StartedEvent{
			JobID:      jobID,
			ProjectDir: plan.ProjectDir,
			StepCount:  len(plan.Steps),
		},
	})

	for index, step := range plan.Steps {
		if ctx.Err() != nil {
			m.finish(jobID, StateCancelled, -1, "Project creation was cancelled.", started)
			return
		}

		m.mu.Lock()
		record.job.StepIndex = index
		m.mu.Unlock()
		m.publish(Event{
			Name: "nodesmith:job:step",
			Payload: StepEvent{
				JobID:  jobID,
				StepID: step.ID,
				Index:  index,
				Total:  len(plan.Steps),
				State:  "running",
			},
		})

		exitCode, runErr := m.runStep(ctx, jobID, step)
		if ctx.Err() != nil {
			m.finish(jobID, StateCancelled, exitCode, "Project creation was cancelled.", started)
			return
		}
		if runErr != nil {
			m.publish(Event{
				Name: "nodesmith:job:step",
				Payload: StepEvent{
					JobID:  jobID,
					StepID: step.ID,
					Index:  index,
					Total:  len(plan.Steps),
					State:  "failed",
				},
			})
			message := fmt.Sprintf("%s failed: %v", step.Label, runErr)
			m.finish(jobID, StateFailed, exitCode, message, started)
			return
		}
		m.publish(Event{
			Name: "nodesmith:job:step",
			Payload: StepEvent{
				JobID:  jobID,
				StepID: step.ID,
				Index:  index,
				Total:  len(plan.Steps),
				State:  "success",
			},
		})
	}

	m.finish(jobID, StateSuccess, 0, "", started)
}

func (m *Manager) runStep(ctx context.Context, jobID string, step planner.PlanStep) (int, error) {
	if step.Kind == planner.StepKindProjectConfig || step.Config != nil {
		if err := writeProjectConfig(step); err != nil {
			return -1, err
		}
		m.appendLog(jobID, step.ID, "stdout", step.Display)
		return 0, nil
	}

	// CommandContext plus an explicit Cancel gives the runtime a deadline it can
	// enforce: WaitDelay force-terminates a process that ignores cancellation.
	// The default Cancel would signal only the direct child, so the whole tree
	// is terminated here instead.
	command := exec.CommandContext(ctx, step.Bin, step.Args...)
	command.Cancel = func() error { return terminateProcess(command) }
	command.WaitDelay = processWaitDelay
	command.Dir = step.Dir
	overrides := make(map[string]string, len(step.Env)+1)
	for key, value := range step.Env {
		if strings.EqualFold(key, "CI") || strings.EqualFold(key, "PATH") {
			continue
		}
		overrides[key] = value
	}
	overrides["CI"] = "true"
	if m.pathProvider != nil {
		pathValue, err := m.pathProvider()
		if err != nil {
			return -1, fmt.Errorf("resolve process PATH: %w", err)
		}
		overrides["PATH"] = pathValue
	}
	command.Env = environ.Merge(os.Environ(), overrides, runtime.GOOS)
	configureProcess(command)

	// Deliberately os.Pipe rather than Cmd.StdoutPipe: Cmd.Wait closes pipes it
	// owns only once every reader has finished, so a grandchild that inherited
	// the write end can block Wait indefinitely. Owning these descriptors lets
	// the step stop reading on a deadline instead of hanging forever.
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return -1, fmt.Errorf("capture stdout: %w", err)
	}
	defer func() { _ = stdoutReader.Close() }()
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutWriter.Close()
		return -1, fmt.Errorf("capture stderr: %w", err)
	}
	defer func() { _ = stderrReader.Close() }()
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter

	startErr := command.Start()
	// The child owns its own descriptors once started. The parent's copies must
	// be closed or the read ends never observe EOF.
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	if startErr != nil {
		return -1, fmt.Errorf("start %s: %w", step.Bin, startErr)
	}

	pumpsDone := make(chan struct{})
	go func() {
		defer close(pumpsDone)
		var pumps sync.WaitGroup
		pumps.Add(2)
		go func() {
			defer pumps.Done()
			m.pump(jobID, step.ID, "stdout", stdoutReader)
		}()
		go func() {
			defer pumps.Done()
			m.pump(jobID, step.ID, "stderr", stderrReader)
		}()
		pumps.Wait()
	}()

	waitErr := command.Wait()

	// The process has exited, but anything it spawned may still hold the write
	// ends open. Collect trailing output briefly, then stop reading so the job
	// always reaches a terminal state.
	grace := time.NewTimer(orphanedOutputGrace)
	select {
	case <-pumpsDone:
		grace.Stop()
	case <-grace.C:
		m.appendLog(
			jobID,
			step.ID,
			"stderr",
			"Nodesmith stopped collecting output: a background process is still holding this step's output stream.",
		)
		_ = stdoutReader.Close()
		_ = stderrReader.Close()
		<-pumpsDone
	}

	if waitErr == nil {
		return 0, nil
	}
	var exitError *exec.ExitError
	if errors.As(waitErr, &exitError) {
		return exitError.ExitCode(), fmt.Errorf("process exited with code %d", exitError.ExitCode())
	}
	if errors.Is(waitErr, exec.ErrWaitDelay) {
		// The process exited on its own; only its descendants overran the delay.
		return 0, nil
	}
	return -1, fmt.Errorf("wait for process: %w", waitErr)
}

func (m *Manager) pump(jobID, stepID, stream string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		m.appendLog(jobID, stepID, stream, scanner.Text())
	}
	// A reader closed on the orphaned-output deadline is an expected stop, not a
	// failure worth reporting to the user.
	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		m.appendLog(jobID, stepID, "stderr", fmt.Sprintf("Nodesmith could not read %s: %v", stream, err))
	}
}

func (m *Manager) appendLog(jobID, stepID, stream, text string) {
	m.mu.RLock()
	record := m.jobs[jobID]
	m.mu.RUnlock()
	if record == nil {
		return
	}

	record.logMu.Lock()
	line := LogLine{
		Seq:    record.nextSeq,
		Stream: stream,
		Text:   text,
		StepID: stepID,
	}
	record.nextSeq++
	record.logs.append(line)
	m.publish(Event{
		Name: "nodesmith:job:log",
		Payload: LogEvent{
			JobID:  jobID,
			Seq:    line.Seq,
			StepID: stepID,
			Stream: stream,
			Text:   text,
		},
	})
	record.logMu.Unlock()
}

func (m *Manager) finish(jobID string, state State, exitCode int, message string, started time.Time) {
	ended := time.Now()
	m.mu.Lock()
	record := m.jobs[jobID]
	if record == nil {
		m.mu.Unlock()
		return
	}
	record.job.State = state
	record.job.ExitCode = exitCode
	record.job.Error = message
	record.job.EndedAt = ended
	projectDir := record.job.ProjectDir
	retention := m.retention
	if m.activeJobID == jobID {
		m.activeJobID = ""
	}
	m.mu.Unlock()

	m.publish(Event{
		Name: "nodesmith:job:done",
		Payload: DoneEvent{
			JobID:      jobID,
			State:      state,
			ExitCode:   exitCode,
			DurationMS: ended.Sub(started).Milliseconds(),
			ProjectDir: projectDir,
			Error:      message,
		},
	})
	if retention > 0 {
		time.AfterFunc(retention, func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			record := m.jobs[jobID]
			if record != nil &&
				isTerminal(record.job.State) &&
				time.Since(record.job.EndedAt) >= retention {
				delete(m.jobs, jobID)
			}
		})
	}
}

func (m *Manager) publish(event Event) {
	m.eventMu.Lock()
	if len(m.eventQueue) >= m.eventLimit {
		switch event.Name {
		case "nodesmith:job:log":
			m.eventMu.Unlock()
			return
		default:
			if !m.discardQueuedLogFromTailLocked() {
				if event.Name != "nodesmith:job:done" {
					m.eventMu.Unlock()
					return
				}
				m.eventQueue[0] = Event{}
				m.eventQueue = m.eventQueue[1:]
			}
		}
	}
	m.eventQueue = append(m.eventQueue, event)
	m.eventMu.Unlock()
	select {
	case m.eventSignal <- struct{}{}:
	default:
	}
}

func (m *Manager) discardQueuedLogFromTailLocked() bool {
	for index := len(m.eventQueue) - 1; index >= 0; index-- {
		if m.eventQueue[index].Name != "nodesmith:job:log" {
			continue
		}
		copy(m.eventQueue[index:], m.eventQueue[index+1:])
		m.eventQueue[len(m.eventQueue)-1] = Event{}
		m.eventQueue = m.eventQueue[:len(m.eventQueue)-1]
		return true
	}
	return false
}

func (m *Manager) dispatch() {
	for range m.eventSignal {
		for {
			m.eventMu.Lock()
			if len(m.eventQueue) == 0 {
				m.eventMu.Unlock()
				break
			}
			event := m.eventQueue[0]
			if len(m.eventQueue) == 1 {
				m.eventQueue = nil
			} else {
				m.eventQueue[0] = Event{}
				m.eventQueue = m.eventQueue[1:]
			}
			m.eventMu.Unlock()

			m.mu.RLock()
			callbacks := make([]subscriber, 0, len(m.subscribers))
			for _, callback := range m.subscribers {
				callbacks = append(callbacks, callback)
			}
			m.mu.RUnlock()
			for _, callback := range callbacks {
				func() {
					defer func() {
						_ = recover()
					}()
					callback(event)
				}()
			}
		}
	}
}

func (m *Manager) pruneLocked(now time.Time) {
	if m.retention == 0 {
		return
	}
	for id, record := range m.jobs {
		if !isTerminal(record.job.State) || record.job.EndedAt.IsZero() {
			continue
		}
		if now.Sub(record.job.EndedAt) >= m.retention {
			delete(m.jobs, id)
		}
	}
}

func isTerminal(state State) bool {
	return state == StateSuccess || state == StateFailed || state == StateCancelled
}

func newJobID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
