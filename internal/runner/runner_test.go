package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"nodesmith/internal/planner"
)

func TestRunnerHelper(t *testing.T) {
	if os.Getenv("GO_WANT_RUNNER_HELPER") != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}
	args := os.Args[separator+1:]
	switch args[0] {
	case "output":
		_, _ = fmt.Fprintln(os.Stdout, "first")
		_, _ = fmt.Fprintln(os.Stderr, "warning")
		_, _ = fmt.Fprint(os.Stdout, "last-without-newline")
	case "fail":
		_, _ = fmt.Fprintln(os.Stderr, "deliberate failure")
		os.Exit(7)
	case "sleep":
		time.Sleep(30 * time.Second)
	case "spawn-child":
		child := exec.Command(
			os.Args[0],
			"-test.run=TestRunnerHelper",
			"--",
			"delayed-marker",
			args[1],
		)
		child.Env = os.Environ()
		if err := child.Start(); err != nil {
			os.Exit(5)
		}
		if err := os.WriteFile(args[2], []byte("started"), 0o600); err != nil {
			os.Exit(6)
		}
		time.Sleep(30 * time.Second)
	case "delayed-marker":
		time.Sleep(2 * time.Second)
		if err := os.WriteFile(args[1], []byte("orphan survived"), 0o600); err != nil {
			os.Exit(7)
		}
	case "flood":
		count, err := strconv.Atoi(args[1])
		if err != nil {
			os.Exit(3)
		}
		for index := 0; index < count; index++ {
			_, _ = fmt.Fprintf(os.Stdout, "line-%05d\n", index)
		}
	case "environment":
		_, _ = fmt.Fprintln(os.Stdout, os.Getenv("PATH"))
	case "protected-environment":
		_, _ = fmt.Fprintln(os.Stdout, os.Getenv("CI"))
		_, _ = fmt.Fprintln(os.Stdout, os.Getenv("PATH"))
	case "read-file":
		content, err := os.ReadFile(args[1])
		if err != nil {
			os.Exit(8)
		}
		_, _ = fmt.Fprint(os.Stdout, string(content))
	default:
		os.Exit(4)
	}
	os.Exit(0)
}

func TestManagerSuccessAndReplay(t *testing.T) {
	manager := NewManager()
	job, err := manager.Start(helperPlan(t, "output"))
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateSuccess {
		t.Fatalf("State = %q, want %q (error: %s)", final.State, StateSuccess, final.Error)
	}
	if final.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", final.ExitCode)
	}

	logs, err := manager.Logs(job.ID, 0)
	if err != nil {
		t.Fatalf("Logs() error = %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("len(Logs()) = %d, want 3: %#v", len(logs), logs)
	}
	texts := make(map[string]bool)
	for index, line := range logs {
		if line.Seq != index {
			t.Fatalf("logs[%d].Seq = %d, want %d", index, line.Seq, index)
		}
		texts[line.Text] = true
	}
	for _, expected := range []string{"first", "warning", "last-without-newline"} {
		if !texts[expected] {
			t.Errorf("Logs() missing %q: %#v", expected, logs)
		}
	}

	tail, err := manager.Logs(job.ID, 2)
	if err != nil {
		t.Fatalf("Logs(fromSeq) error = %v", err)
	}
	if len(tail) != 1 || tail[0].Seq != 2 {
		t.Fatalf("Logs(fromSeq) = %#v, want only seq 2", tail)
	}
}

func TestManagerFailureStopsRemainingSteps(t *testing.T) {
	plan := helperPlan(t, "fail")
	plan.Steps = append(plan.Steps, helperStep(t, "output"))
	manager := NewManager()
	job, err := manager.Start(plan)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateFailed {
		t.Fatalf("State = %q, want %q", final.State, StateFailed)
	}
	if final.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", final.ExitCode)
	}
	if final.StepIndex != 0 {
		t.Fatalf("StepIndex = %d, want 0", final.StepIndex)
	}
	if !strings.Contains(final.Error, "process exited with code 7") {
		t.Fatalf("Error = %q, want exit-code context", final.Error)
	}
}

func TestManagerWritesProjectConfigBeforeTheFollowingCommand(t *testing.T) {
	projectDir := t.TempDir()
	configStep := planner.PlanStep{
		ID:      "minimum-release-age-config",
		Kind:    planner.StepKindProjectConfig,
		Label:   "Write package-manager security policy",
		Dir:     projectDir,
		Env:     map[string]string{},
		Args:    []string{},
		Display: "write pnpm-workspace.yaml: minimumReleaseAge = 4320",
		Config: &planner.ProjectConfig{
			Path:   "pnpm-workspace.yaml",
			Format: planner.ConfigFormatYAML,
			Key:    "minimumReleaseAge",
			Value:  "4320",
		},
	}
	readStep := helperStep(t, "read-file", filepath.Join(projectDir, "pnpm-workspace.yaml"))
	readStep.Dir = projectDir
	manager := NewManager()
	job, err := manager.Start(planner.Plan{
		RecipeID:   "test",
		ProjectDir: projectDir,
		Steps:      []planner.PlanStep{configStep, readStep},
	})
	if err != nil {
		t.Fatal(err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateSuccess {
		t.Fatalf("job = %#v, want success", final)
	}
	logs, err := manager.Logs(job.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 ||
		logs[0].StepID != configStep.ID ||
		!strings.Contains(logs[1].Text, "minimumReleaseAge: 4320") {
		t.Fatalf("logs = %#v, want config write before file-reading command", logs)
	}
}

func TestManagerCancelAndConcurrencyLimit(t *testing.T) {
	manager := NewManager()
	job, err := manager.Start(helperPlan(t, "sleep"))
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForState(t, manager, job.ID, StateRunning, 2*time.Second)

	if _, err := manager.Start(helperPlan(t, "output")); err == nil {
		t.Fatal("second Start() error = nil, want concurrency error")
	}
	if err := manager.Cancel(job.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateCancelled {
		t.Fatalf("State = %q, want %q (error: %s)", final.State, StateCancelled, final.Error)
	}
}

func TestManagerCancellationTerminatesChildProcessTree(t *testing.T) {
	tempDir := t.TempDir()
	marker := filepath.Join(tempDir, "child-finished")
	started := filepath.Join(tempDir, "child-started")
	manager := NewManager()
	job, err := manager.Start(helperPlan(t, "spawn-child", marker, started))
	if err != nil {
		t.Fatal(err)
	}
	waitForFile(t, started, 5*time.Second)
	if err := manager.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateCancelled {
		t.Fatalf("State = %q, want cancelled", final.State)
	}
	time.Sleep(3 * time.Second)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("child marker stat error = %v, want child process tree terminated", err)
	}
}

func TestManagerShutdownCancelsActiveProcess(t *testing.T) {
	manager := NewManager()
	job, err := manager.Start(helperPlan(t, "sleep"))
	if err != nil {
		t.Fatal(err)
	}
	waitForState(t, manager, job.ID, StateRunning, 2*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := manager.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	final, err := manager.Status(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.State != StateCancelled {
		t.Fatalf("State = %q, want cancelled", final.State)
	}
}

func TestManagerRingBuffer(t *testing.T) {
	manager := NewManager(WithRingCapacity(10))
	plan := helperPlan(t, "flood", "150")
	job, err := manager.Start(plan)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateSuccess {
		t.Fatalf("State = %q, want success: %s", final.State, final.Error)
	}
	logs, err := manager.Logs(job.ID, 0)
	if err != nil {
		t.Fatalf("Logs() error = %v", err)
	}
	if len(logs) != 10 {
		t.Fatalf("len(Logs()) = %d, want 10", len(logs))
	}
	if logs[0].Seq != 140 || logs[9].Seq != 149 {
		t.Fatalf("retained seq range = %d..%d, want 140..149", logs[0].Seq, logs[9].Seq)
	}
}

func TestManagerCapturesHundredThousandLines(t *testing.T) {
	manager := NewManager()
	job, err := manager.Start(helperPlan(t, "flood", "100000"))
	if err != nil {
		t.Fatal(err)
	}
	final := waitForTerminal(t, manager, job.ID, 15*time.Second)
	if final.State != StateSuccess {
		t.Fatalf("State = %q, want success: %s", final.State, final.Error)
	}
	logs, err := manager.Logs(job.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != defaultRingCapacity {
		t.Fatalf("retained log count = %d, want %d", len(logs), defaultRingCapacity)
	}
	if logs[0].Seq != 90_000 || logs[len(logs)-1].Seq != 99_999 {
		t.Fatalf(
			"retained seq range = %d..%d, want 90000..99999",
			logs[0].Seq,
			logs[len(logs)-1].Seq,
		)
	}
}

func TestManagerEmitsLifecycleEvents(t *testing.T) {
	manager := NewManager()
	events := make(chan Event, 32)
	unsubscribe := manager.Subscribe(func(event Event) {
		events <- event
	})
	defer unsubscribe()

	job, err := manager.Start(helperPlan(t, "output"))
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateSuccess {
		t.Fatalf("State = %q, want success: %s", final.State, final.Error)
	}

	names := make([]string, 0, 8)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			names = append(names, event.Name)
			if event.Name == "nodesmith:job:done" {
				if len(names) < 4 {
					t.Fatalf("events = %#v, want started, step, output, and done events", names)
				}
				if names[0] != "nodesmith:job:started" {
					t.Fatalf("first event = %q, want nodesmith:job:started", names[0])
				}
				if names[len(names)-1] != "nodesmith:job:done" {
					t.Fatalf("last event = %q, want nodesmith:job:done", names[len(names)-1])
				}
				hasLog := false
				for _, name := range names {
					if name == "nodesmith:job:log" {
						hasLog = true
						break
					}
				}
				if !hasLog {
					t.Fatalf("events = %#v, want at least one log before done", names)
				}
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for done event; events = %#v", names)
		}
	}
}

func TestSlowSubscriberDoesNotBlockRunner(t *testing.T) {
	manager := NewManager()
	unsubscribe := manager.Subscribe(func(Event) {
		time.Sleep(25 * time.Millisecond)
	})
	defer unsubscribe()

	started := time.Now()
	job, err := manager.Start(helperPlan(t, "flood", "5000"))
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateSuccess {
		t.Fatalf("State = %q, want success: %s", final.State, final.Error)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("execution took %s with a slow subscriber, want <= 3s", elapsed)
	}
}

func TestBackpressuredSubscriberUsesBoundedLiveQueueAndReplay(t *testing.T) {
	const liveCapacity = 64
	manager := NewManager(WithEventQueueCapacity(liveCapacity))
	release := make(chan struct{})
	events := make(chan Event, liveCapacity+8)
	unsubscribe := manager.Subscribe(func(event Event) {
		<-release
		events <- event
	})
	defer unsubscribe()

	job, err := manager.Start(helperPlan(t, "flood", "5000"))
	if err != nil {
		t.Fatal(err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateSuccess {
		t.Fatalf("State = %q, want success: %s", final.State, final.Error)
	}
	manager.eventMu.Lock()
	queued := len(manager.eventQueue)
	manager.eventMu.Unlock()
	if queued > liveCapacity {
		t.Fatalf("live event queue length = %d, want <= %d", queued, liveCapacity)
	}
	replayed, err := manager.Logs(job.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 5000 {
		t.Fatalf("replayed log count = %d, want 5000", len(replayed))
	}
	for index, line := range replayed {
		if line.Seq != index {
			t.Fatalf("replayed log %d has seq %d", index, line.Seq)
		}
	}
	close(release)

	names := make([]string, 0, liveCapacity+1)
	logCount := 0
	lastLogSequence := -1
	deadline := time.After(5 * time.Second)
	for {
		select {
		case event := <-events:
			names = append(names, event.Name)
			if event.Name == "nodesmith:job:log" {
				logEvent, ok := event.Payload.(LogEvent)
				if !ok {
					t.Fatalf("log payload = %T, want LogEvent", event.Payload)
				}
				if logEvent.Seq <= lastLogSequence {
					t.Fatalf(
						"retained live log seq = %d after %d, want increasing order",
						logEvent.Seq,
						lastLogSequence,
					)
				}
				lastLogSequence = logEvent.Seq
				logCount++
			}
			if event.Name == "nodesmith:job:done" {
				if logCount == 0 || logCount >= len(replayed) {
					t.Fatalf(
						"received %d bounded live logs, want some but fewer than %d replayed logs",
						logCount,
						len(replayed),
					)
				}
				if names[0] != "nodesmith:job:started" {
					t.Fatalf("first event = %q, want nodesmith:job:started", names[0])
				}
				if names[len(names)-2] != "nodesmith:job:step" {
					t.Fatalf("penultimate event = %q, want successful step event", names[len(names)-2])
				}
				return
			}
		case <-deadline:
			t.Fatalf("timed out after %d events and %d logs", len(names), logCount)
		}
	}
}

func TestManagerErrors(t *testing.T) {
	manager := NewManager()
	if _, err := manager.Start(planner.Plan{}); err == nil {
		t.Fatal("Start(empty) error = nil")
	}
	if _, err := manager.Status("missing"); err == nil {
		t.Fatal("Status(missing) error = nil")
	}
	if _, err := manager.Logs("missing", 0); err == nil {
		t.Fatal("Logs(missing) error = nil")
	}
	if err := manager.Cancel("missing"); err == nil {
		t.Fatal("Cancel(missing) error = nil")
	}
}

func TestManagerExpiresFinishedJobsAfterRetention(t *testing.T) {
	manager := NewManager(WithRetention(25 * time.Millisecond))
	job, err := manager.Start(helperPlan(t, "output"))
	if err != nil {
		t.Fatal(err)
	}
	if final := waitForTerminal(t, manager, job.ID, 5*time.Second); final.State != StateSuccess {
		t.Fatalf("job state = %q, want success", final.State)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := manager.Status(job.ID); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("finished job %q was retained past the configured lifetime", job.ID)
}

func TestMergeEnvironmentReplacesAndSortsOverrides(t *testing.T) {
	got := mergeEnvironment(
		[]string{"PATH=/old", "UNCHANGED=yes", "MALFORMED"},
		map[string]string{"PATH": "/new", "CI": "1"},
	)
	want := []string{"UNCHANGED=yes", "MALFORMED", "CI=1", "PATH=/new"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("mergeEnvironment() = %#v, want %#v", got, want)
	}
}

func TestManagerEnforcesProtectedEnvironment(t *testing.T) {
	trustedPath := filepath.Join(t.TempDir(), "trusted")
	manager := NewManager(WithPathProvider(func() (string, error) {
		return trustedPath, nil
	}))
	plan := helperPlan(t, "protected-environment")
	plan.Steps[0].Env["ci"] = "0"
	plan.Steps[0].Env["Path"] = filepath.Join(t.TempDir(), "untrusted")

	job, err := manager.Start(plan)
	if err != nil {
		t.Fatal(err)
	}
	if final := waitForTerminal(t, manager, job.ID, 5*time.Second); final.State != StateSuccess {
		t.Fatalf("job state = %q, want success: %s", final.State, final.Error)
	}
	logs, err := manager.Logs(job.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].Text != "1" || logs[1].Text != trustedPath {
		t.Fatalf("protected environment logs = %#v, want CI=1 and PATH=%q", logs, trustedPath)
	}
}

func TestManagerUsesCurrentResolvedPathForEveryStep(t *testing.T) {
	pathValue := filepath.Join(t.TempDir(), "first")
	manager := NewManager(WithPathProvider(func() (string, error) {
		return pathValue, nil
	}))

	first, err := manager.Start(helperPlan(t, "environment"))
	if err != nil {
		t.Fatal(err)
	}
	if final := waitForTerminal(t, manager, first.ID, 5*time.Second); final.State != StateSuccess {
		t.Fatalf("first job state = %q, want success: %s", final.State, final.Error)
	}
	firstLogs, err := manager.Logs(first.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstLogs) != 1 || firstLogs[0].Text != pathValue {
		t.Fatalf("first job PATH logs = %#v, want %q", firstLogs, pathValue)
	}

	pathValue = filepath.Join(t.TempDir(), "second")
	second, err := manager.Start(helperPlan(t, "environment"))
	if err != nil {
		t.Fatal(err)
	}
	if final := waitForTerminal(t, manager, second.ID, 5*time.Second); final.State != StateSuccess {
		t.Fatalf("second job state = %q, want success: %s", final.State, final.Error)
	}
	secondLogs, err := manager.Logs(second.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondLogs) != 1 || secondLogs[0].Text != pathValue {
		t.Fatalf("second job PATH logs = %#v, want %q", secondLogs, pathValue)
	}
}

func TestManagerSurfacesPathProviderFailure(t *testing.T) {
	manager := NewManager(WithPathProvider(func() (string, error) {
		return "", fmt.Errorf("discovery failed")
	}))
	job, err := manager.Start(helperPlan(t, "output"))
	if err != nil {
		t.Fatal(err)
	}
	final := waitForTerminal(t, manager, job.ID, 5*time.Second)
	if final.State != StateFailed || !strings.Contains(final.Error, "resolve process PATH") {
		t.Fatalf("final job = %#v, want failed PATH resolution", final)
	}
}

func helperPlan(t *testing.T, mode string, extra ...string) planner.Plan {
	t.Helper()
	tempDir := t.TempDir()
	return planner.Plan{
		RecipeID:   "test",
		ProjectDir: filepath.Join(tempDir, "project"),
		Steps:      []planner.PlanStep{helperStep(t, mode, extra...)},
	}
}

func helperStep(t *testing.T, mode string, extra ...string) planner.PlanStep {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	args := []string{"-test.run=TestRunnerHelper", "--", mode}
	args = append(args, extra...)
	return planner.PlanStep{
		ID:    "helper-" + mode,
		Label: "Helper " + mode,
		Bin:   executable,
		Args:  args,
		Dir:   t.TempDir(),
		Env:   map[string]string{"GO_WANT_RUNNER_HELPER": "1"},
	}
}

func waitForFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		if !os.IsNotExist(err) {
			t.Fatalf("stat %q: %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file %q was not created within %s", path, timeout)
}

func waitForState(t *testing.T, manager *Manager, jobID string, state State, timeout time.Duration) Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := manager.Status(jobID)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if job.State == state {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := manager.Status(jobID)
	t.Fatalf("job did not reach %q within %s; final state %q", state, timeout, job.State)
	return Job{}
}

func waitForTerminal(t *testing.T, manager *Manager, jobID string, timeout time.Duration) Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := manager.Status(jobID)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if isTerminal(job.State) {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := manager.Status(jobID)
	t.Fatalf("job did not finish within %s; final state %q", timeout, job.State)
	return Job{}
}
