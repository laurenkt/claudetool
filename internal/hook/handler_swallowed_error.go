package hook

func init() {
	Register("go-swallowed-error", semgrepHandler(goSwallowedErrorRule))
}

const goSwallowedErrorRule = `rules:
  - id: go-swallowed-error
    patterns:
      - pattern: |
          if err != nil {
              $LOG.$METHOD(...)
          }
      - metavariable-regex:
          metavariable: $LOG
          regex: ^(slog|log|fmt)$
      - pattern-not-regex: "//.*\n"
    languages: [go]
    severity: WARNING
    message: >-
      Error is logged but not propagated. Either return the error
      or add a comment explaining why it is safe to swallow.
`
