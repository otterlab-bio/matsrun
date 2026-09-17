# matsrun

**A Go CLI that turns grouped BAM inputs into reproducible rMATS pairwise splicing runs.**

`matsrun` reads sample groups from an Excel pdata file, discovers BAM inputs, derives read length from SeqKit QC statistics, creates every pairwise group contrast, and invokes `rmats.py` once per contrast and species.

## Proof

One real orchestration run publishes one directory per contrast, with the five rMATS products and a
manifest that records how the pairing was derived:

```text
$ matsrun run --root root --pdata samples.xlsx --seqlengthQC qc --gtf ann.gtf --threads 2

$ ls root/RNASplicing/human/contrast-001/
A3SS.MATS.JC.txt  A5SS.MATS.JC.txt  MXE.MATS.JC.txt  RI.MATS.JC.txt
SE.MATS.JC.txt    matsrun-contrast-manifest.json
```

The manifest is what makes the run auditable after the fact — it names the groups, the read length
taken from the SeqKit statistics, and the exact `rmats.py` invocation (excerpted):

```json
{
  "schema_version": "matsrun.contrast-manifest/v1",
  "species": "human",
  "combination": { "group1": "ctrl", "group2": "treat" },
  "read_length": 150,
  "threads": 2,
  "rmats_version": "rmats.py 4.3.0-stub"
}
```

## Where it fits

```text
RNA-seq BAMs + pdata + SeqKit QC + GTF → matsrun → rmats.py → RNASplicing/<species>/<contrast>
```

The tool orchestrates rMATS; it does not implement the rMATS statistical model.

## Install

```bash
git clone https://github.com/otterlab-bio/matsrun.git
cd matsrun
go build -o matsrun ./cmd/matsrun
./matsrun --help
```

### Prerequisite: rMATS

Execution requires `rmats.py` on `PATH` (matsrun invokes it directly and probes
`rmats.py --version`); an rMATS build supporting `--variable-read-length` is
expected. Without it the build succeeds but every contrast task fails at run
time.

## Quick start

```bash
matsrun run \
  --root /analysis/bam \
  --pdata /analysis/samples.xlsx \
  --seqlengthQC /analysis/qc \
  --gtf /references/hg38.gtf \
  --threads 10
```

For PDX layouts, use `--pdxmode 1` so BAM discovery scans the expected `Filtered_bams/` structure:

```bash
matsrun run \
  --root /analysis/pdx \
  --pdata /analysis/samples.xlsx \
  --seqlengthQC /analysis/qc \
  --gtf /references/hg38.gtf \
  --pdxmode 1
```

## Input contract

- `--root` is the BAM root; ordinary mode scans the root directory and PDX mode scans `Filtered_bams/`.
- `--pdata` is an Excel workbook whose first sheet contains `sampleid` plus `sample_group` or `condition`.
- Column aliases are normalized: `sampleid`/`sample_id`/`样本编号`/`样本ID`, `sample_group`/`group`/`样本分组`/`分组`, and `condition`/`treatment`/`条件`.
- `--seqlengthQC` contains `*_seqkit_stat.txt` files used to derive a common read length.
- `--gtf` is the annotation passed to `rmats.py`.
- At least two non-empty groups are required.

## Output contract

Each species and pairwise group combination is written below:

```text
<root>/RNASplicing/<species>/contrast-001/
└── <rMATS outputs>   # SE/A3SS/A5SS/MXE/RI tables plus manifest.json
```

Each contrast is executed in a staging directory (with a transient `temp/`)
and published atomically after its products are validated; `temp/` never
appears in the published tree. `manifest.json` records the exact `rmats.py`
invocation, the group BAM lists, and the validated product set.

The number of tasks is `number of species × number of pairwise group combinations`. A run returns a non-zero status if one or more contrast tasks fail and reports each failed task.

## Development

```bash
gofmt -w .
go test ./...
go vet ./...
```

## Continuous integration

`.github/workflows/ci.yml` runs on `push`, `pull_request`, and manual dispatch. It checks
`gofmt -l`, runs `go vet` and the full test suite, builds the binary, and verifies CLI
wiring. It then runs a full orchestration smoke with a stubbed `rmats.py` on `PATH`:
pdata Excel discovery, BAM scanning, read-length derivation from SeqKit N50 statistics,
pairwise group contrast generation, product validation, and transactional publication.
The smoke asserts the published contrast directory, the five required rMATS products, and
the `matsrun.contrast-manifest/v1` manifest. rMATS itself is not run in CI because it is
an external heavyweight dependency; the stub only exercises `matsrun`'s own contract.
Logs and outputs are uploaded as the `matsrun-orchestration-evidence` artifact, including
on failure.

## License and repository

MIT · [otterlab-bio/matsrun](https://github.com/otterlab-bio/matsrun)
