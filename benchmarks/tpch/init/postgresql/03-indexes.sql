CREATE INDEX idx_nation_regionkey   ON nation(n_regionkey);
CREATE INDEX idx_supplier_nationkey ON supplier(s_nationkey);
CREATE INDEX idx_partsupp_suppkey   ON partsupp(ps_suppkey);
CREATE INDEX idx_partsupp_partkey   ON partsupp(ps_partkey);
CREATE INDEX idx_customer_nationkey ON customer(c_nationkey);
CREATE INDEX idx_orders_custkey     ON orders(o_custkey);
CREATE INDEX idx_orders_date        ON orders(o_orderdate);
CREATE INDEX idx_lineitem_orderkey  ON lineitem(l_orderkey);
CREATE INDEX idx_lineitem_partkey_suppkey ON lineitem(l_partkey, l_suppkey);
CREATE INDEX idx_lineitem_shipdate  ON lineitem(l_shipdate);

ALTER TABLE nation    ADD FOREIGN KEY (n_regionkey)            REFERENCES region(r_regionkey);
ALTER TABLE supplier  ADD FOREIGN KEY (s_nationkey)            REFERENCES nation(n_nationkey);
ALTER TABLE partsupp  ADD FOREIGN KEY (ps_partkey)             REFERENCES part(p_partkey);
ALTER TABLE partsupp  ADD FOREIGN KEY (ps_suppkey)             REFERENCES supplier(s_suppkey);
ALTER TABLE customer  ADD FOREIGN KEY (c_nationkey)            REFERENCES nation(n_nationkey);
ALTER TABLE orders    ADD FOREIGN KEY (o_custkey)              REFERENCES customer(c_custkey);
ALTER TABLE lineitem  ADD FOREIGN KEY (l_orderkey)             REFERENCES orders(o_orderkey);
ALTER TABLE lineitem  ADD FOREIGN KEY (l_partkey, l_suppkey)   REFERENCES partsupp(ps_partkey, ps_suppkey);

ANALYZE region, nation, part, supplier, partsupp, customer, orders, lineitem;
