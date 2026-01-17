# Gemini CLI Metrics Extraction Core Logic

This document outlines the core logic for extracting usage metrics from a Gemini CLI installation. This specification is designed to be language-agnostic, allowing for the implementation of a similar metrics library in other programming languages (e.g., Python, Rust, Go).

## 1. Data Source Discovery

The Gemini CLI stores session data in a temporary directory within the user's home directory.

- **Root Directory:** `~/.gemini/tmp` (expand `~` to the platform-specific user home).
- **Session File Pattern:** `**/chats/session-*.json` (recursive glob search).
- **Storage Format:** Each session is stored as a JSON file.

## 2. Data Structures

### Raw Session Object
The JSON files follow this approximate structure:

```json
{
  "sessionId": "string (uuid)",
  "projectHash": "string",
  "startTime": "ISO 8601 Timestamp",
  "lastUpdated": "ISO 8601 Timestamp",
  "messages": [
    {
      "id": "string",
      "type": "user | gemini",
      "timestamp": "ISO 8601 Timestamp",
      "model": "string (optional, usually on gemini type)",
      "tokens": {
        "input": number,
        "output": number,
        "cached": number,
        "thoughts": number,
        "total": number
      },
      "toolCalls": [
        {
          "function": { "name": "string" },
          "args": { 
             "file_path": "string",
             "path": "string",
             "dir_path": "string"
          }
        }
      ]
    }
  ]
}
```

## 3. Core Algorithms

### 3.1 Session Merging & Deduplication
Because the CLI may save partial or updated sessions across multiple files, you must merge them by `sessionId`.

1. **Group** all loaded session objects by their `sessionId`.
2. **For each group:**
   - Find the minimum `startTime`.
   - Find the maximum `lastUpdated`.
   - **Deduplicate Messages:** Iterate through all messages in the group. Use `message.id` as a unique key.
     - If multiple versions of the same message ID exist, prefer the one with the most metadata (e.g., the one containing `tokens` or `toolCalls`).
   - **Sort Messages:** Sort the deduplicated messages chronologically by `timestamp`.

### 3.2 Metrics Aggregation
Iterate through the merged sessions and their messages to calculate the following:

- **Total Sessions:** Count of unique `sessionId`s.
- **Total Projects:** Count of unique `projectHash`es.
- **Message Counts:**
    - `totalMessages`: Sum of messages where `type` is `user` or `gemini`.
    - `totalGeminiMessages`: Sum of messages where `type` is `gemini`.
- **Token Usage:** (Only for `type: gemini`)
    - Aggregate `input`, `output`, `cached`, `thoughts`, and `total` tokens.
- **Model Usage:** Maintain a frequency map of `message.model`.
- **Tool Usage:** Count total entries in `message.toolCalls`.
- **Language Inference:**
    - Parse `toolCalls[].args` for file paths.
    - Extract file extensions (e.g., `.ts`, `.py`).
    - Map extensions to language names (e.g., `TypeScript`, `Python`).

### 3.3 Activity Tracking
- **Daily Activity:** Create a map of `YYYY-MM-DD` to message count.
- **Weekday Activity:** Create an array of 7 integers (Sun-Sat) to track message frequency per day of the week.

### 3.4 Streak Calculation
1. **Sorted Dates:** Get all unique dates from the Daily Activity map and sort them.
2. **Max Streak:** Find the longest sequence of consecutive days in the sorted list.
3. **Current Streak:** Starting from "today" or "yesterday", count backwards the number of consecutive days present in the map.

### 3.5 Cost Estimation
Cost is calculated per `gemini` message based on the model and token usage.

1. **Model Matching:** Use a prefix-match or exact-match against a pricing table (see `src/pricing.ts` for reference rates).
2. **Tiered Pricing:** Apply higher rates if `inputTokens` exceeds a threshold (e.g., 200,000).
3. **Calculation:**
   - `CombinedOutput = outputTokens + thoughtsTokens`
   - `FreshInput = max(0, inputTokens - cachedTokens)`
   - `TotalCost = (FreshInput * inputRate) + (CombinedOutput * outputRate) + (cachedTokens * cacheRate)`
   - Rates are typically "per million tokens".

## 4. Language Inference Map
Common mapping for `extension -> Language`:
- `ts, tsx` -> TypeScript
- `js, jsx` -> JavaScript
- `py` -> Python
- `rs` -> Rust
- `go` -> Go
- `rb` -> Ruby
- `java` -> Java
- `cpp, cc, hpp` -> C++
- `md` -> Markdown
- `sh, bash, zsh` -> Shell
- `json` -> JSON
- `yaml, yml` -> YAML
- `sql` -> SQL
