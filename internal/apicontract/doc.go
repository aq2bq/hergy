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
// An operation listed in pending.txt has not been written yet: its schema is
// still the empty one and Checker lets it through. Every other operation must
// say its whole shape — each object closes with additionalProperties: false —
// and the list only gets shorter. A field that is added to an answer without
// being written here makes the tests fail, and so does a field a screen sends
// under a name the server does not read.
//
// It is used by tests only; hergy serve itself never reads the file.
package apicontract
