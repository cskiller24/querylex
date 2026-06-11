#!/bin/bash
set -e

SCALE_FACTOR=${SCALE_FACTOR:-1}
DATA_DIR=${DATA_DIR:-/data}

mkdir -p "$DATA_DIR"

cd /build/tpch-kit/dbgen

echo "Generating TPC-H data at scale factor $SCALE_FACTOR..."
./dbgen -s "$SCALE_FACTOR" -f

echo "Processing .tbl files..."
for f in region nation part supplier partsupp customer orders lineitem; do
    if [ -f "${f}.tbl" ]; then
        sed 's/|$//' "${f}.tbl" > "$DATA_DIR/${f}.tbl"
        echo "  ${f}.tbl: $(wc -l < "$DATA_DIR/${f}.tbl") rows"
    fi
done

echo ""
echo "TPC-H data generation complete (SF=$SCALE_FACTOR)"
echo "Total data size:"
du -sh "$DATA_DIR"
