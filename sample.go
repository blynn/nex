package main
import ("bufio";"./nn";"fmt";"os")
func main() {

  nLines := 0
  nChars := 0
nn_go := func(in *bufio.Reader) {
  nn_ctx := nn.NewContext(bufio.NewReader(os.Stdin))
  for {
    nn_done, nn_action := nn.Next(nn_ctx)
    if nn_done { break }
    switch nn_action {
    case 0:
{ nLines++; nChars++ }
    case 1:
{ nChars++ }
    }
  }
}
  nn_go(bufio.NewReader(os.Stdin))
  fmt.Printf("%d %d\n", nLines, nChars)
}
