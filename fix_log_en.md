# JSON Repair Library Fix Log

## Problem Description

### Original Issue
When using the `json-repair` library to repair the following JSON, the `size` field was lost:

```json
{"items":[{"query":"smart phone","category":["smartphone"],"boost":{"tags":["flagship","5G","high-performance"],"ageGroup":"young_adult","gender":"male","brand":["Apple","Samsung","Google"],"price":{"min":800,"max":1500}},"filter":{"tags":["premium"],"gender":"male","brand":["Apple","Samsung","Google"],"price":{"min":800}}}}],"size":50}
```

### Error Analysis
The JSON has a structural error:
```
..."price":{"min":800}}}}],"size":50
                      ↑↑↑↑
                      4 consecutive }
```

**The correct structure should be:**
```
..."price":{"min":800}}}]}],"size":50
                      ↑↑↑↑↑
                      3 } + 1 ]
```

### Output Before Fix
```json
{"items":[{"boost":{...},"category":["smartphone"],"filter":{...},"query":"smart phone"}]}
```
**Problem: The `size` field was lost!**

---

## Fix Implementation

### Modified File
`jsonrepair.go` - `parseArray()` function

### Specific Changes

#### 1. Added Smart Array End Detection (Lines 237-285)

```go
// Improvement: When encountering '}' in array context, determine if it should end the array
// This fixes errors like "...}}}}]," (extra '}' instead of ']')
c, b = p.getByte(0)
if b && c == '}' {
    // Check if we are in an array context
    isInArrayContext := false
    for _, m := range p.marker {
        if m == "array" {
            isInArrayContext = true
            break
        }
    }
    if isInArrayContext {
        // Lookahead to determine if this '}' should end the array
        // If '}' is followed by '}]' or similar pattern, it might be array end
        // If '}' is followed by ',{' or similar pattern, array should continue
        shouldEndArray := false
        lookahead := 1
        for {
            nextC, nextB := p.getByte(lookahead)
            if !nextB {
                break
            }
            if unicode.IsSpace(rune(nextC)) {
                lookahead++
                continue
            }
            // If '}' is followed by ',' or '{', array should continue
            if nextC == ',' || nextC == '{' {
                shouldEndArray = false
                break
            }
            // If '}' is followed by '}' or ']', it might be array end
            if nextC == '}' || nextC == ']' {
                shouldEndArray = true
                break
            }
            // Other characters (like string start "), array should continue
            break
        }
        if shouldEndArray {
            // Treat '}' as ']' and end the array
            p.index++
            p.resetMarker()
            return rst
        }
    }
}
```

#### 2. Improved Object Value Handling in Arrays (Lines 309-327)

```go
// Improvement: Only break due to '}' when not in array context
// Check marker stack, if 'array' marker exists, we are in array context and should not break
if p.getMarker() == "object_value" && c == '}' {
    // Check if we are in an array context
    isInArrayContext := false
    for _, m := range p.marker {
        if m == "array" {
            isInArrayContext = true
            break
        }
    }
    // Only break when not in array context
    if !isInArrayContext {
        break
    }
    // In array context, skip '}' and continue parsing
    p.index++
    c, b = p.getByte(0)
}
```

---

## Fix Explanation

### 1. Original Code Problem Flow

```
parseArray starts (items array)
    └── Parse 1st item object
        └── Parse filter object
            └── Parse price object: {"min":800}
                └── Encounter } → price object ends ✓
            └── Encounter } → filter object ends ✓
        └── Encounter } → item object ends ✓
    └── Encounter } ← This should be ] to end array!
        └── parseJSON sees } returns ""
        └── value == "" → break
        └── Array ends prematurely, size field not parsed
```

### 2. Fixed Parsing Flow

```
parseArray starts (items array)
    └── Parse 1st item object ✓
    └── Encounter } (1st extra })
        └── Lookahead → Found '}]' pattern
        └── Determine array should end
        └── Treat } as ], return items array
    └── parseObject continues
        └── Parse size: 50 ✓
        └── Encounter } → root object ends
```

### 3. Smart Recognition Algorithm

When encountering `}` in an array, use **lookahead** to determine its semantics:

| Following Character(s) | Decision | Handling |
|----------------------|----------|----------|
| `}` or `]` | Array end | Treat `}` as `]`, end array |
| `,` or `{` | Array continues | Skip `}`, parse next element |
| `"` | Array continues | Skip `}`, parse next element |

### 4. Key Design Decisions

```go
// Example 1: Malformed JSON (needs fixing)
// Input: ...2000}}}}],"size":50
//        Position: 349
// 
// Current char: }
// Lookahead: } (next non-whitespace char)
// Decision: Array end ✓
// Action: Treat } as ], end array, continue parsing size

// Example 2: Normal nested array (should not be affected)
// Input: [ {"a": 1}, {"b": 2} ]
//
// After parsing 1st object:
// Current char: ,
// Decision: Array continues ✓
// Action: Parse 2nd object normally

// Example 3: Normal array end (should not be affected)
// Input: [ {"a": 1} ]
//
// After parsing object:
// Current char: ]
// Action: End array normally
```

---

## Fix Results

### Output After Fix

```json
{
  "items": [
    {
      "query": "smart phone",
      "category": ["smartphone"],
      "boost": {
        "tags": ["flagship", "5G", "high-performance"],
        "ageGroup": "young_adult",
        "gender": "male",
        "brand": ["Apple", "Samsung", "Google"],
        "price": {
          "min": 800,
          "max": 1500
        }
      },
      "filter": {
        "tags": ["premium"],
        "gender": "male",
        "brand": ["Apple", "Samsung", "Google"],
        "price": {
          "min": 800
        }
      }
    }
  ],
  "size": 50
}
```

**✅ The `size` field is successfully preserved!**

### Comparison Test Results

| Feature | Before Fix | After Fix |
|---------|-----------|-----------|
| Repair Success | ✅ | ✅ |
| Result Valid | ✅ | ✅ |
| **Size Field Preserved** | ❌ | ✅ |
| All Original Tests Pass | - | ✅ (59/59) |

---

## Summary

By adding the **Smart Array End Detection** mechanism, the library can now:

1. **Correctly identify** extra `}` in errors like `}}}}`
2. **Convert them to** `}}}]` to properly end arrays
3. **Preserve** all fields after arrays (like `size`)
4. **Not affect** normal JSON parsing

This fix uses **lookahead** technology to achieve intelligent JSON error repair, solving the problem while maintaining backward compatibility.

---

## Testing

All existing tests pass after the fix:

```bash
$ go test -v
=== RUN   Test_RepairJSON
=== RUN   Test_RepairJSON/CASE-1
...
=== RUN   Test_RepairJSON/CASE-59
--- PASS: Test_RepairJSON (0.00s)
    --- PASS: Test_RepairJSON/CASE-1 (0.00s)
    ...
    --- PASS: Test_RepairJSON/CASE-59 (0.00s)
PASS
ok      github.com/RealAlexandreAI/json-repair  0.004s
```

59/59 tests pass.
