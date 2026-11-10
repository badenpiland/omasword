# Building omasword

```bash
go build -o omasword ./cmd/omasword
```

This shows `dev` as the build number. To set one:

```bash
go build -ldflags "-X omasword/internal/version.BuildNumber=123" -o omasword ./cmd/omasword
```

`./build.sh` does the same with an auto-incrementing number stored in
`.build_number` (git-ignored).

The version and build number appear at the right of the header bar and in
`omasword --version`. The version comes from `internal/version/version.go`.

Release builds come from `.github/workflows/release.yml` when a `v*` tag is
pushed; they use the tag as the build number.
