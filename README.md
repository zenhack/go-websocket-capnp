This repository integrates [Go Cap'n Proto][1] with websockets. The main
package uses the [gorilla websocket library][2], and there is a `js`
package which uses browser APIs via `syscall/js`, for use from WASM.

[1]: https://github.com/capnproto/go-capnproto2
[2]: https://github.com/gorilla/websocket
