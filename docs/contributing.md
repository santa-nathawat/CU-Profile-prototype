# Contributing

Use a feature branch and submit a pull request to `main`. Commit one completed, validated step at a time using `type(TICKET): description`, for example `docs(NS-02): record PostgreSQL decision` or `feat(NS-02): add API health endpoint`.

All files are owned by `@santa-nathawat` and `@idealkritarat` on the same CODEOWNERS line. One approving review from either eligible owner satisfies the code-owner requirement; the PR author cannot approve their own PR. Both owners need write access. Approval makes a PR eligible to merge only when required checks and other rules also pass.

The supplied `Protect main` ruleset requires one approval, code-owner review, resolved conversations, and a successful `CI` check on an up-to-date branch. New reviewable commits dismiss stale approvals. It blocks branch deletion and force pushes and declares no bypass actors. Preserve stronger existing repository or organization protections; this ruleset does not override them.

## Repository enforcement

The existing [Protect main ruleset](https://github.com/thinc-org/CU-Profile/settings/rules/26701368) is active and targets `main`. It requires the review and CI rules above. Existing restrictions allowing only squash/rebase merges and requiring an extra approval for unattributed Copilot PRs are preserved in the supplied configuration.

## Reproduce merge enforcement

An administrator can open repository **Settings → Rules → Rulesets → New ruleset → Import a ruleset** and select `.github/rulesets/main.json`. Review the imported configuration and save it as **Active**. Alternatively, after GitHub CLI authentication and inspection of existing protections:

```sh
gh api repos/thinc-org/CU-Profile/rulesets
gh api repos/thinc-org/CU-Profile/rules/branches/main
gh api --method POST repos/thinc-org/CU-Profile/rulesets --input .github/rulesets/main.json
```

Do not create a duplicate ruleset if `Protect main` already exists; inspect and update that ruleset instead. GitHub plan/organization policy may limit available enforcement features. Verify the saved ruleset and check that both owners have write access.

CODEOWNERS is read from a PR's base branch, so owner enforcement starts after this file reaches `main`. For the initial architecture PR, require an eligible human review and successful CI; do not bypass stronger existing rules. No push or merge is part of local scaffold setup.
