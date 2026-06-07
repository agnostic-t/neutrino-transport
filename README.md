![[logo]](./assets/logo.png)

Russian | [English](./README.md)

# Neutrino Project

This repository belongs to the [Neutrino](https://github.com/agnostic-t/neutrino-core) project and is the base implementation of the `transport` module.

## Contents

Currently contains the following implementations:

- [tcp](./basic/tcp/tcp.go): basic transport via TCP connections. This implementation serves as an example for creating other transports (e.g. HTTP).
- [http](./basic/http/http.go): basic transport via HTTP transport. In this implementation server mimicries Nginx server, if used not by VPN client.

Planned transports:

- `udp`: basic transport via UDP (session-like) connections. Useful when using the `quic` multiplexer.
- `https`: A transport that simulates (or uses) HTTPS requests.
- `ws`: A transport that uses WebSocket technology to transfer data.
