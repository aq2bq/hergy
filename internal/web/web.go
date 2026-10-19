// Package web serves hergy's screens over HTTP on this machine: the profile
// screen, the role screen, and the band that walks between them
// (spec-nav.md, spec-profiles.md, spec-roles.md).
//
// What it shows comes from the model, through one place: the catalog the app
// hands in reads every team definition, profile and rule document, and check and
// rules turn them into the sentences and the changes a screen draws. This
// package decides no rule of the model itself; it lays out what those functions
// returned (P1).
//
// The promises it keeps (設計 6 節「残す仕組み」):
//
//   - It listens on 127.0.0.1 only, so nothing off this machine reaches it.
//   - A request whose Host is not this server's own name is refused.
//   - A write is a POST that must carry the token of this run, and an Origin
//     that comes with it must be this server's own.
//   - Every answer carries "Content-Security-Policy: default-src 'self'", so a
//     page cannot pull in script or style from anywhere but this server.
//   - A write reads the file, checks the change, and writes only when the file
//     is still the one it read (source.WriteIfUnchanged).
package web
