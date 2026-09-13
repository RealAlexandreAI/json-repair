package jsonrepair

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// RepairJSONWithSchema is a compatibility helper for schema-guided repair.
func RepairJSONWithSchema(src string, schema map[string]any) (string, error) {
	result, err := RepairWithOptions(src, RepairOptions{Schema: schema})
	return result.JSON, err
}

type schemaRepairer struct {
	root   any
	mode   SchemaRepairMode
	logger []RepairLogEntry
}

func newSchemaRepairer(rawSchema any, mode SchemaRepairMode, logging bool) (*schemaRepairer, error) {
	var logger []RepairLogEntry
	if logging {
		logger = []RepairLogEntry{}
	}
	if _, ok := rawSchema.(map[string]any); ok {
		return &schemaRepairer{root: rawSchema, mode: mode, logger: logger}, nil
	}
	if _, ok := rawSchema.(bool); ok {
		return &schemaRepairer{root: rawSchema, mode: mode, logger: logger}, nil
	}
	return nil, fmt.Errorf("schema must be an object or boolean")
}

func (r *schemaRepairer) log(text, path string) {
	if r.logger == nil {
		return
	}
	r.logger = append(r.logger, RepairLogEntry{Text: text, Context: path})
}

func parseSchemaInput(src string) (value any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("repair json panic: %v", recovered)
		}
	}()

	src = normalizeInput(src)
	if json.Valid([]byte(src)) {
		decoder := json.NewDecoder(strings.NewReader(src))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		return value, nil
	}

	parser := NewJSONParser(src)
	value = parser.parseJSON()
	return parser.collectMultipleTopLevel(value), nil
}

func (r *schemaRepairer) repair(value any, rawSchema any, path string) (any, error) {
	schema, err := r.resolve(rawSchema)
	if err != nil {
		return nil, err
	}
	if allowed, ok := schema.(bool); ok {
		if allowed {
			return cloneSchemaValue(value)
		}
		return nil, fmt.Errorf("schema does not allow value at %s", path)
	}

	schemaObject, ok := schema.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema at %s must be an object or boolean", path)
	}

	if rawBranches, exists := schemaObject["allOf"]; exists {
		branches, ok := rawBranches.([]any)
		if !ok {
			return nil, fmt.Errorf("allOf at %s must be an array", path)
		}
		for _, branch := range branches {
			value, err = r.repair(value, branch, path)
			if err != nil {
				return nil, err
			}
		}
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		rawBranches, exists := schemaObject[key]
		if !exists {
			continue
		}
		branches, ok := rawBranches.([]any)
		if !ok {
			return nil, fmt.Errorf("%s at %s must be an array", key, path)
		}
		var matched any
		matchCount := 0
		for _, branch := range branches {
			candidate, branchErr := r.repair(value, branch, path)
			if branchErr == nil {
				matched = candidate
				matchCount++
			}
		}
		if matchCount == 0 {
			return nil, fmt.Errorf("%s at %s has no matching schema", key, path)
		}
		if key == "oneOf" && matchCount != 1 {
			return nil, fmt.Errorf("oneOf at %s matched %d schemas", path, matchCount)
		}
		value = matched
	}

	types, err := schemaTypes(schemaObject)
	if err != nil {
		return nil, fmt.Errorf("schema at %s: %w", path, err)
	}
	if len(types) > 1 {
		var lastErr error
		for _, schemaType := range types {
			branch := cloneSchemaObject(schemaObject)
			branch["type"] = schemaType
			candidate, branchErr := r.repair(value, branch, path)
			if branchErr == nil {
				return candidate, nil
			}
			lastErr = branchErr
		}
		return nil, lastErr
	}

	schemaType := ""
	if len(types) == 1 {
		schemaType = types[0]
	} else if _, exists := schemaObject["properties"]; exists {
		schemaType = "object"
	} else if _, exists := schemaObject["items"]; exists {
		schemaType = "array"
	}
	if value == "" && schemaType != "" && schemaType != "string" {
		if inferred, ok := inferMissingSchemaValue(schemaObject); ok {
			value = inferred
			r.log("filled a missing value from the schema", path)
		}
	}

	var repaired any
	switch schemaType {
	case "object":
		repaired, err = r.repairObject(value, schemaObject, path)
	case "array":
		repaired, err = r.repairArray(value, schemaObject, path)
	case "string", "integer", "number", "boolean", "null":
		repaired, err = coerceSchemaScalar(value, schemaType, path)
		if err == nil {
			r.log("applied schema scalar coercion", path)
		}
	default:
		repaired, err = cloneSchemaValue(value)
	}
	if err != nil {
		return nil, err
	}

	if constant, exists := schemaObject["const"]; exists && !schemaValuesEqual(repaired, constant) {
		return nil, fmt.Errorf("value at %s does not match const", path)
	}
	if enum, exists := schemaObject["enum"]; exists {
		values, ok := enum.([]any)
		if !ok {
			return nil, fmt.Errorf("enum at %s must be an array", path)
		}
		matched := false
		for _, allowed := range values {
			if schemaValuesEqual(repaired, allowed) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("value at %s does not match enum", path)
		}
	}
	if err := validateSchemaConstraints(repaired, schemaObject, path); err != nil {
		return nil, err
	}
	return repaired, nil
}

func (r *schemaRepairer) repairObject(value any, schema map[string]any, path string) (map[string]any, error) {
	if r.mode == SchemaRepairSalvage {
		if items, ok := value.([]any); ok {
			if mapped, mappedOK := r.mapArrayToObject(items, schema, path); mappedOK {
				value = mapped
				r.log("mapped array to object by schema property order", path)
			} else if path == "$" && len(items) == 1 {
				if object, objectOK := items[0].(map[string]any); objectOK {
					value = object
					r.log("unwrapped a single-item root array to object", path)
				}
			}
		}
	}
	if text, ok := value.(string); ok {
		parsed, err := parseSchemaInput(text)
		if err != nil {
			return nil, fmt.Errorf("object at %s: %w", path, err)
		}
		value = parsed
		r.log("unwrapped a JSON string to an object", path)
	}

	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object at %s, got %T", path, value)
	}

	properties, err := schemaProperties(schema, path)
	if err != nil {
		return nil, err
	}
	repaired := make(map[string]any, len(object))

	for key, propertySchema := range properties {
		propertyPath := path + "." + key
		rawValue, exists := object[key]
		if exists {
			repaired[key], err = r.repair(rawValue, propertySchema, propertyPath)
			if err != nil {
				return nil, err
			}
			continue
		}
		if filled, ok, fillErr := r.fillMissing(propertySchema, propertyPath); fillErr != nil {
			return nil, fillErr
		} else if ok {
			repaired[key], err = r.repair(filled, propertySchema, propertyPath)
			if err != nil {
				return nil, err
			}
			r.log("filled a missing property from the schema", propertyPath)
		}
	}

	required, err := schemaRequired(schema, path)
	if err != nil {
		return nil, err
	}
	for _, key := range required {
		if _, exists := repaired[key]; exists {
			continue
		}
		return nil, fmt.Errorf("missing required property %s at %s", key, path)
	}

	additional := schema["additionalProperties"]
	patterns, err := schemaPatternProperties(schema, path)
	if err != nil {
		return nil, err
	}
	for key, rawValue := range object {
		if _, exists := properties[key]; exists {
			continue
		}
		propertyPath := path + "." + key
		matchedPattern := false
		for pattern, patternSchema := range patterns {
			matched, matchErr := regexp.MatchString(pattern, key)
			if matchErr != nil {
				return nil, fmt.Errorf("invalid patternProperties regex %q: %w", pattern, matchErr)
			}
			if matched {
				repaired[key], err = r.repair(rawValue, patternSchema, propertyPath)
				if err != nil {
					return nil, err
				}
				matchedPattern = true
			}
		}
		if matchedPattern {
			continue
		}
		switch value := additional.(type) {
		case bool:
			if !value {
				r.log("dropped a property disallowed by the schema", propertyPath)
				continue
			}
			repaired[key], err = cloneSchemaValue(rawValue)
		case map[string]any:
			repaired[key], err = r.repair(rawValue, value, propertyPath)
		default:
			repaired[key], err = cloneSchemaValue(rawValue)
		}
		if err != nil {
			return nil, err
		}
	}

	if maxProperties, ok := schemaInteger(schema["maxProperties"]); ok && len(repaired) > maxProperties {
		return nil, fmt.Errorf("object at %s exceeds maxProperties", path)
	}
	if minProperties, ok := schemaInteger(schema["minProperties"]); ok && len(repaired) < minProperties {
		return nil, fmt.Errorf("object at %s does not meet minProperties", path)
	}
	return repaired, nil
}

func (r *schemaRepairer) repairArray(value any, schema map[string]any, path string) ([]any, error) {
	if text, ok := value.(string); ok {
		parsed, err := parseSchemaInput(text)
		if err != nil {
			return nil, fmt.Errorf("array at %s: %w", path, err)
		}
		value = parsed
		r.log("unwrapped a JSON string to an array", path)
	}

	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array at %s, got %T", path, value)
	}

	repaired := make([]any, 0, len(items))
	itemSchema, hasItemSchema := schema["items"]
	tupleSchemas, isTuple := itemSchema.([]any)
	for index, item := range items {
		if !hasItemSchema {
			repaired = append(repaired, item)
			continue
		}
		activeItemSchema := itemSchema
		if isTuple {
			if index < len(tupleSchemas) {
				activeItemSchema = tupleSchemas[index]
			} else {
				switch additionalItems := schema["additionalItems"].(type) {
				case bool:
					if !additionalItems {
						r.log("dropped an extra tuple item disallowed by the schema", fmt.Sprintf("%s[%d]", path, index))
						continue
					}
				case map[string]any:
					activeItemSchema = additionalItems
				default:
					repaired = append(repaired, item)
					continue
				}
			}
		}
		itemValue, err := r.repair(item, activeItemSchema, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			if r.mode == SchemaRepairSalvage {
				r.log("dropped an array item that did not satisfy the schema", fmt.Sprintf("%s[%d]", path, index))
				continue
			}
			return nil, err
		}
		repaired = append(repaired, itemValue)
	}

	if minItems, ok := schemaInteger(schema["minItems"]); ok && len(repaired) < minItems {
		return nil, fmt.Errorf("array at %s does not meet minItems", path)
	}
	if maxItems, ok := schemaInteger(schema["maxItems"]); ok && len(repaired) > maxItems {
		return nil, fmt.Errorf("array at %s exceeds maxItems", path)
	}
	return repaired, nil
}

func (r *schemaRepairer) fillMissing(rawSchema any, path string) (any, bool, error) {
	schema, err := r.resolve(rawSchema)
	if err != nil {
		return nil, false, err
	}
	if allowed, ok := schema.(bool); ok {
		if !allowed {
			return nil, false, nil
		}
		return nil, false, nil
	}

	schemaObject, ok := schema.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("schema at %s must be an object or boolean", path)
	}
	for _, key := range []string{"const", "default"} {
		if value, exists := schemaObject[key]; exists {
			cloned, cloneErr := cloneSchemaValue(value)
			r.log("filled a missing value from the schema", path)
			return cloned, true, cloneErr
		}
	}
	if enum, exists := schemaObject["enum"]; exists {
		values, ok := enum.([]any)
		if !ok || len(values) == 0 {
			return nil, false, fmt.Errorf("enum at %s must contain at least one value", path)
		}
		cloned, cloneErr := cloneSchemaValue(values[0])
		r.log("filled a missing value from the schema enum", path)
		return cloned, true, cloneErr
	}
	return nil, false, nil
}

func (r *schemaRepairer) resolve(rawSchema any) (any, error) {
	current := rawSchema
	seen := make(map[string]bool)
	for depth := 0; depth < 32; depth++ {
		schema, ok := current.(map[string]any)
		if !ok {
			return current, nil
		}
		ref, ok := schema["$ref"].(string)
		if !ok {
			return current, nil
		}
		if seen[ref] {
			return nil, fmt.Errorf("circular schema reference %q", ref)
		}
		seen[ref] = true
		if !strings.HasPrefix(ref, "#/") {
			return nil, fmt.Errorf("unsupported schema reference %q", ref)
		}
		root, ok := r.root.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("schema reference %q requires an object root schema", ref)
		}
		resolved, err := resolveSchemaReference(root, strings.TrimPrefix(ref, "#/"))
		if err != nil {
			return nil, err
		}
		current = resolved
	}
	return nil, fmt.Errorf("schema reference nesting is too deep")
}

func inferMissingSchemaValue(schema map[string]any) (any, bool) {
	if value, exists := schema["const"]; exists {
		return value, true
	}
	if values, ok := schema["enum"].([]any); ok && len(values) > 0 {
		return values[0], true
	}
	if value, exists := schema["default"]; exists {
		return value, true
	}
	schemaType, _ := schemaTypes(schema)
	if len(schemaType) != 1 {
		return nil, false
	}
	switch schemaType[0] {
	case "string":
		return "", true
	case "integer", "number":
		return 0, true
	case "boolean":
		return false, true
	case "array":
		return []any{}, true
	case "object":
		return map[string]any{}, true
	case "null":
		return nil, true
	default:
		return nil, false
	}
}

func (r *schemaRepairer) mapArrayToObject(value []any, schema map[string]any, path string) (map[string]any, bool) {
	properties, err := schemaProperties(schema, path)
	if err != nil || len(properties) == 0 || len(properties) != len(value) {
		return nil, false
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	mapped := make(map[string]any, len(keys))
	for index, key := range keys {
		mapped[key] = value[index]
	}
	return mapped, true
}

func resolveSchemaReference(root map[string]any, path string) (any, error) {
	var current any = root
	for _, part := range strings.Split(path, "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("schema reference %q is not an object path", path)
		}
		var exists bool
		current, exists = object[part]
		if !exists {
			return nil, fmt.Errorf("unresolvable schema reference %q", path)
		}
	}
	return current, nil
}

func schemaTypes(schema map[string]any) ([]string, error) {
	typeValue, exists := schema["type"]
	if !exists {
		return nil, nil
	}
	switch value := typeValue.(type) {
	case string:
		return []string{value}, nil
	case []any:
		types := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("type must contain strings")
			}
			types = append(types, text)
		}
		return types, nil
	default:
		return nil, fmt.Errorf("type must be a string or array")
	}
}

func schemaProperties(schema map[string]any, path string) (map[string]any, error) {
	properties, exists := schema["properties"]
	if !exists {
		return map[string]any{}, nil
	}
	result, ok := properties.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("properties at %s must be an object", path)
	}
	return result, nil
}

func schemaPatternProperties(schema map[string]any, path string) (map[string]any, error) {
	patterns, exists := schema["patternProperties"]
	if !exists {
		return map[string]any{}, nil
	}
	result, ok := patterns.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("patternProperties at %s must be an object", path)
	}
	return result, nil
}

func validateSchemaConstraints(value any, schema map[string]any, path string) error {
	switch value := value.(type) {
	case string:
		if minimum, ok := schemaInteger(schema["minLength"]); ok && len([]rune(value)) < minimum {
			return fmt.Errorf("string at %s does not meet minLength", path)
		}
		if maximum, ok := schemaInteger(schema["maxLength"]); ok && len([]rune(value)) > maximum {
			return fmt.Errorf("string at %s exceeds maxLength", path)
		}
		if pattern, ok := schema["pattern"].(string); ok {
			matched, err := regexp.MatchString(pattern, value)
			if err != nil {
				return fmt.Errorf("invalid schema pattern at %s: %w", path, err)
			}
			if !matched {
				return fmt.Errorf("string at %s does not match pattern", path)
			}
		}
	case []any:
		if unique, ok := schema["uniqueItems"].(bool); ok && unique {
			for left := 0; left < len(value); left++ {
				for right := left + 1; right < len(value); right++ {
					if schemaValuesEqual(value[left], value[right]) {
						return fmt.Errorf("array at %s does not satisfy uniqueItems", path)
					}
				}
			}
		}
	case json.Number, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		number, ok := schemaNumericValue(value)
		if !ok {
			return fmt.Errorf("number at %s is not finite", path)
		}
		if minimum, ok := schemaNumericValue(schema["minimum"]); ok && number < minimum {
			return fmt.Errorf("number at %s is below minimum", path)
		}
		if maximum, ok := schemaNumericValue(schema["maximum"]); ok && number > maximum {
			return fmt.Errorf("number at %s exceeds maximum", path)
		}
		if minimum, ok := schemaNumericValue(schema["exclusiveMinimum"]); ok && number <= minimum {
			return fmt.Errorf("number at %s is not above exclusiveMinimum", path)
		}
		if maximum, ok := schemaNumericValue(schema["exclusiveMaximum"]); ok && number >= maximum {
			return fmt.Errorf("number at %s is not below exclusiveMaximum", path)
		}
	}
	return nil
}

func schemaRequired(schema map[string]any, path string) ([]string, error) {
	required, exists := schema["required"]
	if !exists {
		return nil, nil
	}
	switch values := required.(type) {
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("required at %s must contain strings", path)
			}
			result = append(result, text)
		}
		return result, nil
	case []string:
		return values, nil
	default:
		return nil, fmt.Errorf("required at %s must be an array", path)
	}
}

func coerceSchemaScalar(value any, schemaType string, path string) (any, error) {
	switch schemaType {
	case "string":
		switch value := value.(type) {
		case string:
			return value, nil
		case json.Number:
			return value.String(), nil
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return fmt.Sprint(value), nil
		default:
			return nil, fmt.Errorf("expected string at %s, got %T", path, value)
		}
	case "integer":
		integer, ok := schemaIntegerValue(value)
		if !ok {
			return nil, fmt.Errorf("expected integer at %s, got %T", path, value)
		}
		return integer, nil
	case "number":
		number, ok := schemaNumberValue(value)
		if !ok {
			return nil, fmt.Errorf("expected number at %s, got %T", path, value)
		}
		return number, nil
	case "boolean":
		boolean, ok := schemaBooleanValue(value)
		if !ok {
			return nil, fmt.Errorf("expected boolean at %s, got %T", path, value)
		}
		return boolean, nil
	case "null":
		if value == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("expected null at %s, got %T", path, value)
	default:
		return nil, fmt.Errorf("unsupported schema type %q at %s", schemaType, path)
	}
}

func schemaInteger(value any) (int, bool) {
	integer, ok := schemaIntegerValue(value)
	if !ok || integer < 0 || int64(int(integer)) != integer {
		return 0, false
	}
	return int(integer), true
}

func schemaIntegerValue(value any) (int64, bool) {
	switch value := value.(type) {
	case int:
		return int64(value), true
	case int8:
		return int64(value), true
	case int16:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	case uint:
		return int64(value), uint64(value) <= math.MaxInt64
	case uint8:
		return int64(value), true
	case uint16:
		return int64(value), true
	case uint32:
		return int64(value), true
	case uint64:
		return int64(value), value <= math.MaxInt64
	case float32:
		return integerFromFloat(float64(value))
	case float64:
		return integerFromFloat(value)
	case json.Number:
		if integer, err := strconv.ParseInt(value.String(), 10, 64); err == nil {
			return integer, true
		}
		parsed, err := strconv.ParseFloat(value.String(), 64)
		if err != nil {
			return 0, false
		}
		return integerFromFloat(parsed)
	case string:
		if integer, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
			return integer, true
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, false
		}
		return integerFromFloat(parsed)
	default:
		return 0, false
	}
}

func integerFromFloat(value float64) (int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value || value < math.MinInt64 || value > math.MaxInt64 {
		return 0, false
	}
	return int64(value), true
}

func schemaNumberValue(value any) (any, bool) {
	switch value := value.(type) {
	case bool, nil:
		return nil, false
	case json.Number:
		if _, err := strconv.ParseFloat(value.String(), 64); err != nil {
			return nil, false
		}
		return value, true
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return value, true
	case float32:
		return float64(value), !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
	case float64:
		return value, !math.IsNaN(value) && !math.IsInf(value, 0)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	default:
		return nil, false
	}
}

func schemaBooleanValue(value any) (bool, bool) {
	if boolean, ok := value.(bool); ok {
		return boolean, true
	}
	if text, ok := value.(string); ok {
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "true", "yes", "y", "on", "1":
			return true, true
		case "false", "no", "n", "off", "0":
			return false, true
		}
	}
	if number, ok := schemaNumericValue(value); ok {
		if number == 0 {
			return false, true
		}
		if number == 1 {
			return true, true
		}
	}
	return false, false
}

func cloneSchemaObject(schema map[string]any) map[string]any {
	clone := make(map[string]any, len(schema)+1)
	for key, value := range schema {
		clone[key] = value
	}
	return clone
}

func cloneSchemaValue(value any) (any, error) {
	switch value := value.(type) {
	case nil, bool, string, json.Number,
		int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return value, nil
	case []any:
		clone := make([]any, len(value))
		for index, item := range value {
			var err error
			clone[index], err = cloneSchemaValue(item)
			if err != nil {
				return nil, err
			}
		}
		return clone, nil
	case map[string]any:
		clone := make(map[string]any, len(value))
		for key, item := range value {
			var err error
			clone[key], err = cloneSchemaValue(item)
			if err != nil {
				return nil, err
			}
		}
		return clone, nil
	default:
		return nil, fmt.Errorf("value of type %T is not JSON-compatible", value)
	}
}

func schemaValuesEqual(left, right any) bool {
	leftNumber, leftIsNumber := schemaNumericValue(left)
	rightNumber, rightIsNumber := schemaNumericValue(right)
	if leftIsNumber && rightIsNumber {
		return leftNumber == rightNumber
	}

	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func schemaNumericValue(value any) (float64, bool) {
	switch value := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseFloat(value.String(), 64)
		return parsed, err == nil
	case int:
		return float64(value), true
	case int8:
		return float64(value), true
	case int16:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint:
		return float64(value), true
	case uint8:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint64:
		return float64(value), true
	case float32:
		return float64(value), !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
	case float64:
		return value, !math.IsNaN(value) && !math.IsInf(value, 0)
	default:
		return 0, false
	}
}
