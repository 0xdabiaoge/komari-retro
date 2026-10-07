# Experimental metric storage

This package is an experimental storage and rollup implementation. The production server does not initialize it, write to it, or query it. Its dialect helpers do not mean the main application supports MySQL or PostgreSQL.

Production load history uses the SQLite `records` / `records_long_term` tables. Ping statistics use `ping_records`. The admin page keeps active database maintenance controls and hides the unconnected DSN, rollup, retention and migration controls.

Before enabling this package, implement and validate dual writes, historical backfill, reconciliation, a feature-controlled query switch and rollback. Do not expose experimental settings as effective production controls before those steps are complete.
