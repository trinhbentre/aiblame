# Security policy

aiblame runs `git` locally and reads your repository history. It does not
contact the network unless you explicitly pass a URL or `owner/repo`, in which
case it runs `git clone`/`git fetch` on that remote and nothing else.

The optional `prepare-commit-msg` hook installed by `aiblame hook install` is a
plain POSIX shell script you can read with `aiblame hook print`. It only
appends a trailer to the commit message file it is given.

## Reporting a vulnerability

Please open a private security advisory on GitHub
(Security → Advisories → "Report a vulnerability") or e-mail the maintainer
listed in the repository profile. Expect an acknowledgement within a week.

## Supported versions

Only the latest 1.x release receives fixes.
