---
name: aiclean
description: Clean and refactor code with a focus on reducing lines of code while improving readability.
---

When this skill is used: (Read the `AGENTS.md` for project guidelines)

1. check the last commits in this branch if any, as well as the uncommited changes in the current checkout.
2. analyze the files involved and resolve the following tasks:

- define and assign variables in the very same line if possible (prefer `bool a = true;` instead of `bool a; a=true`
- be positive for if/else the conditionals (if must be the true/valid case)
- do not use goto statements, refactor long functions into smaller logic
- main objective is to reduce LOCs and make the code more readable and clean
- use r2 string apis instead of glib/libc ones
- prefer simpler, shorter logic
- do surgically well thought patches
