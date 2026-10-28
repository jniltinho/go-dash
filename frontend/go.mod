// This file only marks frontend/ as a separate Go module so that
// `go test ./...` and `go vet ./...` at the repository root skip
// node_modules (some npm packages ship .go files). There is no Go code here.
module dashboard/frontend

go 1.26.0
