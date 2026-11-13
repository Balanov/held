# Security policy

## Scope

HELD reads files. It does not run your model, your agent or your robot, it does
not open sockets, it does not write outside the paths you name on the command
line, and it has no telemetry.

## Reporting a vulnerability

Open a private security advisory on the repository, or email the maintainer.
Please include a contract, a scores file and the command you ran. A reduced
case is worth more than a description.

## What HELD does not do

- It does not authenticate anything. The scores file is evidence, not proof of
  origin. If your scores can be forged, the verdict can be forged with them.
- It does not sign its receipts. A receipt contains the SHA-256 of the contract
  and of the data file, so you can detect accidental edits and verify the files
  you were given. It is not a signature and does not prove who produced the
  data.
- It does not check that the held-out split really was never used for
  selection. It cannot. That is why the contract carries the pin: the pin
  tells a reviewer that the protocol has not changed, and the split labels are
  a claim the split owner has to make honestly.
