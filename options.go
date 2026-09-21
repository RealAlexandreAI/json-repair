package jsonrepair

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
)

// SchemaRepairMode controls how schema-guided repair handles values that do
// not satisfy the schema.
type SchemaRepairMode string

const (
	SchemaRepairStandard SchemaRepairMode = "standard"
	SchemaRepairSalvage  SchemaRepairMode = "salvage"
)

// RepairOptions controls library behavior. The zero value is the normal
// repair path; advanced behavior is opt-in for headless callers.
type RepairOptions struct {
	SkipJSONValidation bool
	Logging            bool
	StreamStable       bool
	Strict             bool
	Schema             any
	SchemaRepairMode   SchemaRepairMode
}

// RepairLogEntry records one repair decision and the nearby input context.
type RepairLogEntry struct {
	Text    string
	Context string
}

// RepairResult contains both the repaired Go value and its compact JSON form.
// Log is empty unless RepairOptions.Logging is true.
type RepairResult struct {
	Value any
	JSON  string
	Log   []RepairLogEntry
}

// RepairWithOptions is the library entry point for advanced repair behavior.
// Existing callers can continue using RepairJSON or MustRepairJSON.
func RepairWithOptions(src string, options RepairOptions) (result RepairResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = RepairResult{}
			err = fmt.Errorf("repair json panic: %s", debug.Stack())
		}
	}()

	mode, err := normalizeSchemaRepairMode(options.SchemaRepairMode)
	if err != nil {
		return RepairResult{}, err
	}
	if options.Schema == nil && mode == SchemaRepairSalvage {
		return RepairResult{}, fmt.Errorf("schema repair mode %q requires a schema", mode)
	}
	if options.Schema != nil && options.Strict {
		return RepairResult{}, fmt.Errorf("schema and strict cannot be used together")
	}

	src = normalizeInput(src)
	logs := []RepairLogEntry(nil)
	if options.Logging {
		logs = []RepairLogEntry{}
	}

	if !options.SkipJSONValidation && !options.Strict && json.Valid([]byte(src)) {
		value, decodeErr := decodeJSONValue(src)
		if decodeErr != nil {
			return RepairResult{}, decodeErr
		}
		if options.Schema != nil {
			repairer, repairerErr := newSchemaRepairer(options.Schema, mode, options.Logging)
			if repairerErr != nil {
				return RepairResult{Log: logs}, repairerErr
			}
			value, repairerErr = repairer.repair(value, options.Schema, "$")
			if repairerErr != nil {
				return RepairResult{Log: logs}, repairerErr
			}
			bs, marshalErr := JSONMarshal(value)
			if marshalErr != nil {
				return RepairResult{Log: logs}, marshalErr
			}
			if options.Logging {
				logs = append(logs, repairer.logger...)
			}
			return RepairResult{Value: value, JSON: strings.TrimSpace(string(bs)), Log: logs}, nil
		}
		compact := &bytes.Buffer{}
		if compactErr := json.Compact(compact, []byte(src)); compactErr != nil {
			return RepairResult{}, compactErr
		}
		return RepairResult{Value: value, JSON: compact.String(), Log: logs}, nil
	}
	if !options.SkipJSONValidation && options.Strict && options.Schema == nil && json.Valid([]byte(src)) {
		validator := newJSONParser(src, parserOptions{
			logging: true,
			strict:  true,
		})
		validated := validator.parseJSON()
		validator.collectMultipleTopLevel(validated)
		if validator.err != nil {
			return RepairResult{Log: validator.logger}, validator.err
		}
		value, decodeErr := decodeJSONValue(src)
		if decodeErr != nil {
			return RepairResult{}, decodeErr
		}
		compact := &bytes.Buffer{}
		if compactErr := json.Compact(compact, []byte(src)); compactErr != nil {
			return RepairResult{}, compactErr
		}
		if !options.Logging {
			validator.logger = nil
		}
		return RepairResult{Value: value, JSON: compact.String(), Log: validator.logger}, nil
	}

	parser := newJSONParser(src, parserOptions{
		logging:      options.Logging,
		streamStable: options.StreamStable,
		strict:       options.Strict,
	})
	value := parser.parseJSON()
	value = parser.collectMultipleTopLevel(value)
	if parser.err != nil {
		return RepairResult{Log: parser.logger}, parser.err
	}

	if options.Schema != nil {
		repairer, repairerErr := newSchemaRepairer(options.Schema, mode, options.Logging)
		if repairerErr != nil {
			return RepairResult{Log: parser.logger}, repairerErr
		}
		if mode == SchemaRepairSalvage && len(parser.topLevelValues) > 1 {
			value, repairerErr = repairTopLevelFragments(repairer, parser.topLevelValues)
		} else {
			value, repairerErr = repairer.repair(value, options.Schema, "$")
		}
		if repairerErr != nil {
			return RepairResult{Log: parser.logger}, repairerErr
		}
		if options.Logging {
			parser.logger = append(parser.logger, repairer.logger...)
		}
	}

	bs, marshalErr := JSONMarshal(value)
	if marshalErr != nil {
		return RepairResult{Log: parser.logger}, marshalErr
	}
	if options.Logging {
		logs = parser.logger
	}
	return RepairResult{Value: value, JSON: strings.TrimSpace(string(bs)), Log: logs}, nil
}

// RepairValue parses and repairs JSON-like input into a Go value.
func RepairValue(src string) (any, error) {
	result, err := RepairWithOptions(src, RepairOptions{})
	return result.Value, err
}

// RepairReader repairs all data read from reader.
func RepairReader(reader io.Reader, options RepairOptions) (RepairResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return RepairResult{}, err
	}
	return RepairWithOptions(string(data), options)
}

// RepairFile repairs a file without changing it on disk.
func RepairFile(filename string, options RepairOptions) (result RepairResult, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return RepairResult{}, err
	}
	defer func() {
		if cerr := file.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	return RepairReader(file, options)
}

func decodeJSONValue(src string) (any, error) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(src))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func normalizeSchemaRepairMode(mode SchemaRepairMode) (SchemaRepairMode, error) {
	if mode == "" {
		return SchemaRepairStandard, nil
	}
	if mode != SchemaRepairStandard && mode != SchemaRepairSalvage {
		return "", fmt.Errorf("schema repair mode must be %q or %q", SchemaRepairStandard, SchemaRepairSalvage)
	}
	return mode, nil
}

func repairTopLevelFragments(repairer *schemaRepairer, fragments []any) (any, error) {
	var lastErr error
	for _, fragment := range fragments {
		value, err := repairer.repair(fragment, repairer.root, "$")
		if err == nil {
			return value, nil
		}
		repairer.log("skipped a top-level fragment that did not match the schema", "$")
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return "", nil
}
