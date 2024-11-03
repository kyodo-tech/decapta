# decapta

**decapta** is a command-line tool for managing structured data files (such as ARB localization files and CSV datasets) in conjunction with [Decap CMS](https://decapcms.org/). It automates the conversion, configuration, and synchronization of these data files, making it easy to edit and manage them through Decap CMS.

- **Pre-process**: Converts ARB and CSV files into a Decap CMS-compatible content structure, enabling in-CMS data editing.
- **Config generation**: Automatically generates or upserts a Decap CMS `config.yml` to match the data structure, saving time and ensuring consistency.
- **Post-process**: Converts CMS-edited content files back to their original ARB or CSV formats, for deployment or application use.

The pre-process and config generation steps are separated to allow for manual editing while usually being updated during continuous integration.

File formats can be added by extending the cli flags in `main.go` and implementing a corresponding module with these three steps.

## Installation

Clone the repository and build the tool:

```bash
go install github.com/kyodo-tech/decapta@latest
```

## Usage

Example usage to manage Flutter ARB localization files:

```sh
# Create content files from ARB data
decapta pre-process -t arb -i ~/flutter/myapp/lib/src/localization
decapta config -t arb -i ~/flutter/myapp/lib/src/localization
# Output the arb files to the data directory
decapta post-process -t arb -o ~/flutter/myapp/lib/src/localization
```

Example usage to manage CSV data:

```sh
# Create content files from CSV data
decapta pre-process -t csv -i ../_data
decapta config -t csv -i ../_data
# Output the csv files to the data directory
decapta post-process -t csv -o ../_data
```

NOTE: CSV files are currently expected to have headers.

## Multiple Projects from a Single CMS

With `decapta`, you can centralize the management of multiple data projects within a single CMS instance by organizing each project’s data and content in a structured directory layout. This setup allows to simplify editing across various datasets or localization files without needing separate CMS instances for each project. To enable this, the `config` step will upsert collections based on their folder path.

### Suggested Directory Structure

In this structure, each project has a dedicated subdirectory under `./data` and `./content`:

```plaintext
.
├── admin/
│   └── config.yml       # Central CMS configuration
├── data/                # Project data files
│   ├── project1/
│   │   └── data.csv
│   └── project2/
│       └── data.arb
└── content/             # CMS-compatible content files for each project
    ├── project1/
    └── project2/
```

The structure in `content/` should mirror the one in `data/`, enabling easy mapping between data files and their CMS-compatible versions.

In our setup, a CI step selectively runs the post-process step for edited projects and pushes resulting data files to a corresponding upstream repository.

## Internals

### Decapta ID Field

For stable content traceability, `decapta` generates a unique identifier for each data entry, labeled `decapta_id`. This field, which defaults to the field name `slug`, allows each entry to be uniquely identified across CMS edits and data exports.

You can define which fields are used to construct `decapta_id` by specifying the `--slug` option during the `pre-process` step. The `--slug` flag accepts a comma-separated list of fields (e.g., `--slug id,name,status`), which `decapta` will concatenate to form a unique identifier.

The `decapta_id` field is stripped out of the final CSV export during `post-process`, maintaining consistency with the original data format.

### Preserving Column Order in CSV

Original column order from the source CSV is maintained throughout import and export steps. During pre-process, `decapta` saves the order of columns in a `.<collectionname>.yaml` file. This ordering is restored in the post-process step, ensuring the final CSV output matches the original structure for consistency and compatibility with downstream applications.

### Reserved Fields

Certain field names (e.g., `data`) are reserved in Decap CMS. During pre-processing and in the config step, these fields are prefixed with `decapta_` (e.g., `data` becomes `decapta_data`). This prefix is automatically removed during post-processing, restoring the original field names in CSV outputs.

### Multi-Line Content in CSV and YAML

YAML handles multi-line text in two styles:
- **Literal Block Style (`|`)** preserves line breaks exactly as written.
- **Folded Block Style (`>-`)** collapses consecutive lines into a flowable paragraph, inserting spaces between lines.

Decap CMS may auto-adjust between these styles based on content format. However, `decapta`'s post-processing restores original text formatting by removing YAML-specific artifacts, ensuring clean multi-line content in CSV exports.
