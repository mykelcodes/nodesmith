package recipe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

func Decode(reader io.Reader) (Manifest, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return Manifest{}, fmt.Errorf("read recipe JSON: %w", err)
	}
	if !utf8.Valid(data) {
		return Manifest{}, fmt.Errorf("decode recipe: JSON is not valid UTF-8")
	}
	if err := rejectDuplicateObjectKeys(data); err != nil {
		return Manifest{}, fmt.Errorf("decode recipe: %w", err)
	}

	var manifest Manifest
	if err := decodeStrict(bytes.NewReader(data), &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode recipe: %w", err)
	}
	return manifest, nil
}

func DecodeBytes(data []byte) (Manifest, error) {
	return Decode(bytes.NewReader(data))
}

func DecodeAndValidate(reader io.Reader) (Manifest, error) {
	manifest, err := Decode(reader)
	if err != nil {
		return Manifest{}, err
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, fmt.Errorf("validate recipe: %w", err)
	}
	return manifest, nil
}

func decodeStrict(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected data after JSON value")
		}
		return fmt.Errorf("decode trailing data: %w", err)
	}
	return nil
}

func rejectDuplicateObjectKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var walk func(string) error
	walk = func(path string) error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, isDelimiter := token.(json.Delim)
		if !isDelimiter {
			return nil
		}

		switch delimiter {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("%s: object key is not a string", path)
				}
				if _, duplicate := seen[key]; duplicate {
					return fmt.Errorf("%s.%s: duplicate JSON object key %q", path, key, key)
				}
				seen[key] = struct{}{}
				if err := walk(path + "." + key); err != nil {
					return err
				}
			}
			if _, err := decoder.Token(); err != nil {
				return err
			}
		case '[':
			index := 0
			for decoder.More() {
				if err := walk(fmt.Sprintf("%s[%d]", path, index)); err != nil {
					return err
				}
				index++
			}
			if _, err := decoder.Token(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: unexpected JSON delimiter %q", path, delimiter)
		}
		return nil
	}

	return walk("$")
}
