@../../specs/run/README.md

- keep the specifications up to date as you make changes
- never run 'go build' without specifying some output directory in /tmp or similar-- it polutes the working directory. remember that 'go test' _internally_ runs 'go build', and so tests whether a build would succeed.
