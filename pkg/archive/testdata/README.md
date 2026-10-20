Password fixtures contain no private data. Tests do not need archive command-line tools.

- `password.zip`: generated with `zip -P secret`, member `secret.txt` contains `secret contents`.
- `password.7z`, `password-headers.7z`: generated with `7zz a -psecret`, respectively `-mhe=off` and `-mhe=on`, same member/content.
- `rar*.rar`: from https://github.com/markokr/rarfile/tree/master/test/files (ISC license), password `password`.
  RAR5 fixtures contain `stest1.txt` / `stest2.txt` (2048 bytes each); RAR3 fixtures contain empty `file1.txt` / `file2.txt`.

RAR fixture source license (https://github.com/markokr/rarfile/blob/master/LICENSE):

Copyright (c) 2005-2026 Marko Kreen <markokr@gmail.com>

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.