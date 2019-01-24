workflow "lint and test" {
  on = "push"
  resolves = ["test"]
}

action "lint" {
  uses = "docker://golangci/golangci-lint"
}

action "test" {
  uses = "docker://golang:latest"
  needs = ["lint"]
  args = "go test -v -race ./..."
}
