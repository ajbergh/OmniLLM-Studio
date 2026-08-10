# Assistant Profile Portability

OmniLLM-Studio Assistant Profiles can be exported as versioned JSON bundles and imported into another installation without carrying local ownership metadata or credentials.

## Bundle contract

The initial interchange format uses:

- `schema`: `omnillm.assistant-profile`
- `version`: `1`
- `profile`: portable profile name, description, optional provider/model hints, system instructions, and allowed tool names
- `skills`: the Markdown content of Skills attached to the profile, in attachment order
- `warnings`: optional export warnings, such as a previously deleted attached Skill

The bundle intentionally excludes profile/Skill database IDs, owner IDs, workspace IDs, timestamps, provider API keys, OpenAPI credentials, MCP secrets, and any other encrypted provider data.

## Export behavior

`GET /v1/assistant-profiles/{id}/export` is owner-scoped and returns the portable bundle. Attached Skills are resolved under the same owner before being embedded. If a profile references a Skill that no longer exists, the missing Skill is omitted and a warning is included.

Export files contain system instructions and Skill Markdown. Treat those instructions as potentially sensitive content and review a bundle before sharing it externally.

## Import behavior

`POST /v1/assistant-profiles/import` accepts only the current schema/version and enforces the same practical size limits used by profile and Skill editing. Import is transactional:

1. validate the complete bundle before writes;
2. generate new local IDs for every Skill;
3. create Skills under the authenticated local owner with no imported workspace ownership;
4. generate a new profile ID and attach the newly created Skill IDs;
5. commit the profile and Skills together.

A failed import does not intentionally reuse or trust source installation IDs. Provider and model strings are portable hints only; the destination still needs an appropriate enabled provider/model configuration.

## Compatibility

Future incompatible bundle changes should increment `version`. Import must reject unknown schema/version combinations rather than guessing how to interpret them.
