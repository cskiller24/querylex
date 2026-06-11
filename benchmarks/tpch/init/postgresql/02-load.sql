\COPY region    FROM '/tpch-data/region.tbl'    WITH (FORMAT csv, DELIMITER '|');
\COPY nation    FROM '/tpch-data/nation.tbl'    WITH (FORMAT csv, DELIMITER '|');
\COPY part      FROM '/tpch-data/part.tbl'      WITH (FORMAT csv, DELIMITER '|');
\COPY supplier  FROM '/tpch-data/supplier.tbl'  WITH (FORMAT csv, DELIMITER '|');
\COPY partsupp  FROM '/tpch-data/partsupp.tbl'  WITH (FORMAT csv, DELIMITER '|');
\COPY customer  FROM '/tpch-data/customer.tbl'  WITH (FORMAT csv, DELIMITER '|');
\COPY orders    FROM '/tpch-data/orders.tbl'    WITH (FORMAT csv, DELIMITER '|');
\COPY lineitem  FROM '/tpch-data/lineitem.tbl'  WITH (FORMAT csv, DELIMITER '|');
