# Update

Update an existing OBTC node checkout from its configured remote and rebuild:

```bash
cd /path/to/obtcd
git pull --ff-only
go build ./...
go install -v . ./cmd/...
```
