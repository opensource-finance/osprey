# Documentation Style

All Osprey documentation uses ASD-STE100 Simplified Technical English (STE).
STE makes the text easy to read and easy to translate.

`make docs-lint` checks the rules that a script can measure.
Run it before you open a pull request.
To show an incorrect example, put it between `<!-- docs-lint: off -->` and `<!-- docs-lint: on -->`.

## Rules

| Rule | What to do | Checked by `make docs-lint` |
|---|---|---|
| Sentence length | Procedures: 20 words or fewer. Descriptions: 25 words or fewer. | Yes (25 maximum) |
| One topic | Write one topic in each sentence. | No |
| Paragraph length | Write 6 sentences or fewer in each paragraph. | Yes |
| One instruction | Write one instruction in each sentence. | No |
| Imperative | Write instructions as commands: "Run `make test`." | No |
| Condition first | Put the condition before the instruction: "If the port is busy, stop the server." | No |
| Active voice | Use the active voice. Use the passive voice only in descriptions, and only if it is necessary. | No |
| Verb tenses | Use the simple present, simple past, or future tense. | No |
| No "-ing" verbs | Do not use the "-ing" form of a verb, except in a technical name. | No |
| Articles | Do not omit "the", "a", and "an". | No |
| Noun clusters | Use 3 nouns or fewer in a noun cluster. | No |
| Lists | Use a vertical list for steps or for 3 or more items. | No |
| Punctuation | Do not use semicolons or em dashes. | Yes |
| Approved words | Use the words in the table below. | Yes, for the words in the table |
| Safety | Start a warning or a caution with a command. Then give the risk. | No |

STE permits technical names and technical verbs.
Examples are `CEL`, `tenant`, `typology`, `compile`, and `deploy`.
Code, commands, field names, and tables are not prose.
The checker does not measure them.

## Words

These are common substitutions.
The official ASD-STE100 dictionary is the full reference.

| Do not use | Use |
|---|---|
| utilize, leverage | use |
| ensure | make sure |
| assist | help |
| approximately | about |
| in order to | to |
| via | through |
| initiate | start |
| terminate | stop |
| prior to | before |
| subsequent, subsequently | after, then |
| sufficient | enough |
| additional | more |
| e.g. | for example |
| i.e. | that is |
| etc. | (give the full list) |

## Safety Instructions

Use these labels:

- **WARNING:** a risk of data loss, a security exposure, or a production outage.
- **CAUTION:** a risk of damage to the configuration or the test data.
- **NOTE:** more information.

Put the label before the step that it applies to.

Example:

> **WARNING:** Do not share the admin token. The token gives write access to all rules.

## Example

Not STE:

<!-- docs-lint: off -->
> Prior to running the integration tests, you'll want to ensure that the server is up and listening on port 8080, which can be done via `go run ./cmd/osprey`.
<!-- docs-lint: on -->

STE:

> 1. Start the server: `go run ./cmd/osprey`.
> 2. Make sure that the server listens on port 8080.
> 3. Run the integration tests.
