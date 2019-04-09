workflow "build, lint, and test" {
  on = "push"
  resolves = ["test", "lint"]
}

action "build" {
  uses = "docker://golang:latest"
  args = "script/build"
}

action "lint" {
  uses = "docker://golangci/golangci-lint"
  args = "script/lint"
}

action "test" {
  uses = "docker://golang:latest"
  args = "script/test"
}
