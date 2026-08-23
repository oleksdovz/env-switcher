# US2 validation

- [x] F2 cancellation discloses no file content and confirmation returns the complete unmasked file.
- [x] Editor precedence, argument parsing, missing editor, and failed editor are tested.
- [x] Invalid reload retains the prior model; valid changed-function reload replaces it and warns.
- [x] Acknowledgement metadata is digest-only, `0600`, atomic, and fails closed on unsafe permissions.
- [x] Loading, validation, hashing, warning acknowledgement, and reload execute no configured function.
