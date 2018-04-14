[WIP] go-assuan
===========

Pure Go implementation of Assuan IPC protocol.

Assuan protocol is used in GnuPG for communication between following
components: gpg, gpg-agent, pinentry, dirmngr. All of them are running as
separate processes and need a way to talk with each other. Assuan solves this
problem. 

Using this library you can talk to gpg-agent or dirmngr directly, invoke
pinentry to get password prompt similar to GnuPG's one and even use Assuan as a
protocol for your own IPC needs.

Assuan documentation: https://www.gnupg.org/documentation/manuals/assuan/index.html

Usage
-------

*quick introduction and example should go here*

License
---------

MIT.
