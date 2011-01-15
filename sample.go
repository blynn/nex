package main
import ("bufio";"./nn";"os")
func main() {
  nn_ctx := nn.NewContext(bufio.NewReader(os.Stdin))
  for {
    nn_done, nn_action := nn.Next(nn_ctx)
    if nn_done { break }
    switch nn_action {
    case -1:
    default:
      println("executing", nn_action)
    }
  }
}
