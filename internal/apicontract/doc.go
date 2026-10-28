// Package apicontract holds the WebUI's API to its one written promise,
// openapi.yaml (n-e3de, d-e206): the screens and the server are bound to that
// file, not to each other's code.
//
// The file is the source. Go's answers are checked against it while the tests
// run: Checker wraps the server's handler, and every request a test sends to
// /api/ and every answer it gets back is validated, the request body and the
// answer alike. The TypeScript types the screens read are made from the same
// file (api-types.d.ts, made by tools/webtools and checked by the apitypes
// gate).
//
// Every operation says its whole shape — each object closes with
// additionalProperties: false, and no schema is the empty one. A route added
// to the server is written here in the same change: a field added to an answer
// without being written makes the tests fail, and so does a field a request
// sends under a name the server does not read. A request the server refused is
// a test's to send on purpose, so only the answer to it is held.
//
// It is used by tests only; hergy serve itself never reads the file.
package apicontract
