workflow "lint and test" {
  on = "push"
  resolves = ["test"]
}

action "build" {
  uses = "docker://golang:latest"
  args = "script/build"
}

action "lint" {
  uses = "docker://golangci/golangci-lint"
  args = "golangci-lint run"
}

action "test" {
  uses = "docker://golang:latest"
  args = "script/test"
  needs = ["build"]
}
