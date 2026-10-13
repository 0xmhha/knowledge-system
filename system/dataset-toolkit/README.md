# Dataset toolkit compatibility directory

Current installation and indexing use [cks setup](../docs/SETUP.md) or [guided setup](../../tools/setup/README.md). Versioned identity, locking, candidate verification and managed rollback are owned by the Go setup implementation.

The scripts retained in this directory are compatibility helpers for earlier workflows. Their presence does not mean they provide the current immutable v2 contract or replace coordinated setup. Use current executable help and the shared configuration rather than old multi-repository manuals. [Versioned operations](../docs/ops-blue-green-reindex.md) describes refresh and rollback; [remaining scope](../../docs/REMAINING-WORK.md) separates external host/operator tasks.
