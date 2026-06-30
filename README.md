# nraa-enduro-workflows

**nraa-enduro-workflows** provides an Enduro preprocessing workflow for NRAA
SIPs.

- [Configuration](#configuration)
- [Preprocessing workflow](#preprocessing-workflow)
- [Local environment](#local-environment)
- [Makefile](#makefile)

## Configuration

Example configuration for the default `nraa-enduro-worker`:

```toml
debug = false
verbosity = 0

[temporal]
address = "temporal-frontend.enduro-sdps:7233"
namespace = "default"

[worker]
taskQueue = "nraa-enduro"
maxConcurrentSessions = 1

[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"

[preprocessing.bagCreate]
checksumAlgorithm = "sha512"

[preprocessing.removeFiles]
removeNames = ["thumbs.db", ".DS_Store"]

[preprocessing.fileFormat]
# Configure at most one of these paths.
# allowlistPath = "/home/enduro/config/allowed-file-formats.csv"
# disallowlistPath = "/home/enduro/config/disallowed-file-formats.csv"

[preprocessing.fileValidate.veraPDF]
# path = "/usr/bin/verapdf"
```

`removeNames` is used by the remove-files activity to delete unneeded files
before validation. This deletion happens before structure, manifest, and
checksum checks run. There is no default `removeNames` value. File
format policy validation is optional; set either `allowlistPath` or
`disallowlistPath`, never both. veraPDF validation is enabled only when
`preprocessing.fileValidate.veraPDF.path` is set.

### Enduro

The child workflow section for Enduro's configuration:

```toml
[[childWorkflows]]
type = "preprocessing"
namespace = "default"
taskQueue = "nraa-enduro"
workflowName = "preprocessing"
extract = true
sharedPath = "/home/enduro/preprocessing"
```

## Preprocessing workflow

The NRAA preprocessing workflow verifies the zipped SIP size, extracts the
package, deletes configured unneeded files before validation, identifies the SIP,
validates the NRAA structure and SIP name, verifies the metadata manifest and
checksums, checks file format policy, validates file formats, validates
`header/metadata.xml` against `header/metadata.xsd`, stops on collected content
errors, then writes PREMIS XML, restructures the SIP, writes `identifiers.json`,
and bags the SIP for Enduro.

## Local environment

The supported development workflow is to run `tilt up` from the Enduro
repository and load this repository through Enduro's `CHILD_WORKFLOW_PATHS`
mechanism.

Bring up the Enduro environment by following the [Enduro development manual].

### Set up

The specific requirements for this project are:

- clone this repository as a sibling of the Enduro repository
- configure `CHILD_WORKFLOW_PATHS=../nraa-enduro-workflows`
- configure `MOUNT_PREPROCESSING_VOLUME=true`
- run `tilt up` from the Enduro repository

All other development workflow details, including `.tilt.env`, live updates,
starting, stopping and clearing the environment, are documented in Enduro. This
repository can also provide local overrides through its own `.tilt.env` file,
including settings such as `TRIGGER_MODE_AUTO`.

### Requirements for development

While we run the services inside a Kubernetes cluster we recommend installing Go
and other tools locally to ease the development process.

- [Go] (1.26+)
- GNU [Make] and [GCC]

## Makefile

The Makefile provides developer utility scripts via command line `make` tasks.
Running `make` with no arguments (or `make help`) prints the help message.
Dependencies are downloaded automatically.

### Debug mode

The debug mode produces more output, including the commands executed. E.g.:

```shell
$ make env DBG_MAKEFILE=1
Makefile:10: ***** starting Makefile for goal(s) "env"
Makefile:11: ***** Fri 10 Nov 2023 11:16:16 AM CET
go env
GO111MODULE=''
GOARCH='amd64'
...
```

[Enduro development manual]: https://enduro.readthedocs.io/dev-manual/devel/
[go]: https://go.dev/doc/install
[make]: https://www.gnu.org/software/make/
[gcc]: https://gcc.gnu.org/
