package main
import ("bufio";"./nn";"os")
func main() {
  nn_ctx := nn.NewContext(bufio.NewReader(os.Stdin))
  for !nn.IsDone(nn_ctx) {
    switch action := nn.Iterate(nn_ctx); action {
    case -1:
    default:
      println("executing", action)
    }
  }
}
