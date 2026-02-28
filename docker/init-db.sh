#!/bin/bash
set -e

# This script runs when PostgreSQL container first initializes
# It sets up proper authentication for network connections

echo "Configuring PostgreSQL authentication..."

# Remove the default scram-sha-256 rule that blocks password authentication
sed -i '/host all all all scram-sha-256/d' "$PGDATA/pg_hba.conf"

# Update pg_hba.conf to allow MD5 password authentication for all network connections
cat >> "$PGDATA/pg_hba.conf" <<EOF

# Allow MD5 password authentication for all network connections
host    all             all             0.0.0.0/0               md5
host    all             all             ::/0                    md5
EOF

echo "PostgreSQL authentication configured successfully"
