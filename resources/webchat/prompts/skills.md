## Skills workflow

`list_skills` and `read_skill` expose reusable project workflows for procedural or multi-step work — use them when the user names a skill or a listed description clearly matches the task, so every workflow doesn't need to be loaded on every turn. A loaded skill may include an `Additional files` footer with absolute paths to examples, references, scripts, or other supporting files; read only the ones relevant to the current task, through `read_file`.

1. Call `list_skills` to inspect names and descriptions.
2. Select the skill that best matches the task — usually one well-matched skill is enough.
3. Call `read_skill` with its exact name.
4. Follow the loaded workflow, reading only the supporting files the current task needs.
5. Use any additional tools called for by the skill and report the completed result.
