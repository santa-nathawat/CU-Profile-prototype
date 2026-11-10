---
status: accepted
---

# Use PostgreSQL for persistence

Use PostgreSQL because teams, memberships, roles, and ratings have relational constraints and transactional updates. MongoDB would require more deliberate management of references and duplicated data. Schema, database driver, ORM, and migrations remain deferred until database design; this skeleton does not connect to the database.
