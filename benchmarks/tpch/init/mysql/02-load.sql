USE tpch;

SET FOREIGN_KEY_CHECKS = 0;
SET UNIQUE_CHECKS = 0;
SET autocommit = 0;

LOAD DATA INFILE '/tpch-data/region.tbl'
    INTO TABLE region
    FIELDS TERMINATED BY '|'
    LINES TERMINATED BY '\n'
    (r_regionkey, r_name, r_comment);

LOAD DATA INFILE '/tpch-data/nation.tbl'
    INTO TABLE nation
    FIELDS TERMINATED BY '|'
    LINES TERMINATED BY '\n'
    (n_nationkey, n_name, n_regionkey, n_comment);

LOAD DATA INFILE '/tpch-data/part.tbl'
    INTO TABLE part
    FIELDS TERMINATED BY '|'
    LINES TERMINATED BY '\n'
    (p_partkey, p_name, p_mfgr, p_brand, p_type, p_size, p_container, p_retailprice, p_comment);

LOAD DATA INFILE '/tpch-data/supplier.tbl'
    INTO TABLE supplier
    FIELDS TERMINATED BY '|'
    LINES TERMINATED BY '\n'
    (s_suppkey, s_name, s_address, s_nationkey, s_phone, s_acctbal, s_comment);

LOAD DATA INFILE '/tpch-data/partsupp.tbl'
    INTO TABLE partsupp
    FIELDS TERMINATED BY '|'
    LINES TERMINATED BY '\n'
    (ps_partkey, ps_suppkey, ps_availqty, ps_supplycost, ps_comment);

LOAD DATA INFILE '/tpch-data/customer.tbl'
    INTO TABLE customer
    FIELDS TERMINATED BY '|'
    LINES TERMINATED BY '\n'
    (c_custkey, c_name, c_address, c_nationkey, c_phone, c_acctbal, c_mktsegment, c_comment);

LOAD DATA INFILE '/tpch-data/orders.tbl'
    INTO TABLE orders
    FIELDS TERMINATED BY '|'
    LINES TERMINATED BY '\n'
    (o_orderkey, o_custkey, o_orderstatus, o_totalprice, o_orderdate, o_orderpriority, o_clerk, o_shippriority, o_comment);

LOAD DATA INFILE '/tpch-data/lineitem.tbl'
    INTO TABLE lineitem
    FIELDS TERMINATED BY '|'
    LINES TERMINATED BY '\n'
    (l_orderkey, l_partkey, l_suppkey, l_linenumber, l_quantity, l_extendedprice, l_discount, l_tax, l_returnflag, l_linestatus, l_shipdate, l_commitdate, l_receiptdate, l_shipinstruct, l_shipmode, l_comment);

COMMIT;

SET FOREIGN_KEY_CHECKS = 1;
SET UNIQUE_CHECKS = 1;
SET autocommit = 1;
