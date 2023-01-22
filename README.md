This repository integrates [Go Cap'n Proto][1] with websockets. The main
package uses [github.com/gobwas/ws](https://github.com/gobwas/ws), and
there is a `js` package which uses browser APIs via `syscall/js`, for
use from WASM.

[1]: https://github.com/capnproto/go-capnproto2
