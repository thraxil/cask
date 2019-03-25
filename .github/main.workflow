workflow "run go test on push" {
  on = "push"
  resolves = ["test"]
}

action "test" {
  uses = "thraxil/cask/actions/action-go-test@master"
}
