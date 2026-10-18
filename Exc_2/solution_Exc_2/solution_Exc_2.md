# Exercise 2 - Operation Data Plane with OLTPs
**Done by:** Deyan Dobrev - S2510929006



## Part 2 - Activities
### Activity 2.1 — PostgreSQL Analytical Queries (E-commerce)

**SQL code used for the 'orders' table**:
```sql
DROP TABLE IF EXISTS orders;

CREATE TABLE orders(
    id SERIAL PRIMARY KEY,
    customer_name TEXT,
    product_category TEXT,
    quantity INTEGER,
    price_per_unit DECIMAL(9, 2),
    order_date DATE,
    country TEXT
);

\COPY orders(customer_name, product_category, quantity, price_per_unit, order_date, country) FROM '/data/orders_1M.csv' DELIMITER ',' CSV HEADER;
```

**Answers to the following questions:**
#### - **A.** Which order has the highest `price_per_unit`?
```
Order 841292, made by Emma Brown on 11.10.2024 has the highest 'price_per_unit': 2000.00. It is in the 'Automotive' category with quantity of 3 units. The country is Italy.
```
**SQL query used for this question:**
```sql
SELECT *
FROM orders
ORDER BY price_per_unit DESC
LIMIT 1;
```

#### - **B.** What are the top 3 product categories with the highest total quantity sold across all orders?
```
The top 3 product categories are: 
- 1. Health and Beauty(300842 units) 
- 2. Electronics(300804 units)
- 3. Toys(300598 units)
```
**SQL query used for this question:**
```sql
SELECT product_category, SUM(quantity) AS quantity_total
FROM orders
GROUP BY product_category
ORDER BY quantity_total DESC
LIMIT 3;
```

#### - **C.** What is the total revenue per product category?(Revenue = `price_per_unit × quantity`)
```
The total revenue per product category is:
- Books: 12731976.04
- Grocery: 15268355.66
- Toys: 23271039.02
- Fashion: 31566368.22
- Office Supplies: 38276061.64
- Health and Beauty: 46599817.89
- Sports: 61848990.83
- Home and Garden: 78023780.09
- Electronics: 241525009.45
- Automotive: 306589798.86
```
**SQL query used for this question:**
```sql
SELECT product_category, SUM(price_per_unit * quantity) AS revenue_total
FROM orders
GROUP BY product_category
ORDER BY revenue_total;
```

#### - **D.** Who are the top 5 customers by total spending?
```
The top 5 customers by total spending are:
- Carol Taylor: 991179.18
- Nina Lopez: 975444.95
- Daniel Jackson: 959344.48
- Carol Lewis: 947708.57
- Daniel Young: 946030.14
```
**SQL query used for this question:**
```sql
SELECT customer_name, SUM(price_per_unit * quantity) AS spending_total
FROM orders
GROUP BY customer_name
ORDER BY spending_total DESC
LIMIT 5;
```

#### - **E.** Look at the spending totals in D — and at how many orders each of those customers has. What do you notice? Open ecommerce/dataset_generator.py and explain why the data looks like this.
```
The reason why the data looks like this is, because we have 1056 total possible customers(32 first names * 33 last names) and 1 000 000 orders. Every customer has around 947 orders(because 1 000 000/1056) and almost the identical total spending, because of the uniform random sampling. This means that these top 5 people are just statistical noise and not really rich buyers.
```
**SQL query used for this question:**
```sql
SELECT customer_name, SUM(price_per_unit * quantity) AS spending_total, COUNT(*) AS orders_total
FROM orders
GROUP BY customer_name
ORDER BY spending_total DESC
LIMIT 5;
```


### Activity 2.2 — PostgreSQL Analytical Queries (E-commerce)
#### Step 1 — Measure how it grows.
| rows in table | join result (`COUNT(*)`) | time |
|---|---|---|
| 50 000 | 27501822 |  3.09 sec |
| 100 000 | 109946508 | 10.32 sec |
| 200 000 | 439395606 | 60.94 sec |
**SQL query used for this step:**
```sql
CREATE TABLE people_50k  AS SELECT * FROM people_big WHERE id <= 50000;
CREATE TABLE people_100k AS SELECT * FROM people_big WHERE id <= 100000;
CREATE TABLE people_200k AS SELECT * FROM people_big WHERE id <= 200000;

SELECT COUNT(*) FROM people_50k p1 JOIN people_50k p2 ON p1.country = p2.country;

SELECT COUNT(*) FROM people_100k p1 JOIN people_100k p2 ON p1.country = p2.country;

SELECT COUNT(*) FROM people_200k p1 JOIN people_200k p2 ON p1.country = p2.country;
```

```
When the input size doubles, the count and the runtime grow by a factor of 4(which means O(N^2) quadratic complexity).

So the prediction for 'people_big'(1M) is:
- count: 439395606 * 5^2 = 10984890150
- time : 60.94 * 5^2 = 1523.5 sec(25.39 min)
```

#### Step 2 — Does an index help?
**SQL query used for this step:**
```sql
CREATE INDEX index_100k_people ON people_100k(country);
ANALYZE people_100k;

EXPLAIN ANALYZE 
SELECT COUNT(*) 
FROM people_100k p1 
JOIN people_100k p2 
  ON p1.country = p2.country;
```
```
The index doesn't help in optimizing the join query. It is because index is used for speeding the data retrieval from a table, but here we don't search for specific or matching rows, but we join and count millions of matching pairs. That is why the index cannot reduce the quadratic complexity(O(N^2)) to a linear complexity(O(N)).
```

#### Step 3 - Rewrite it.
**SQL query used for this step:**
```sql
SELECT SUM(count * count)
FROM (
    SELECT COUNT(*) AS count
    FROM people_100k
    GROUP BY country
);
```
```
After testing this query on the 'people_big' dataset, I got:
- count: 10983941260(compared to my prediction: 10984890150)
- time: 0.6079 sec(compared to my prediction: 1523.5 sec(25.39 min)).
```

#### Step 4 - Discussion.
```
- Adding more hardware is not efficient as rewriting the query from quadratic to linear complexity. This improves the performance, without requiring adding more hardware or indexes.

- Bigger machine would help, but this help would be minimal, because transmitting aorund 100 billion rows is inefficient, because the network can act as a bottleneck.

- OLTP databases are used for short transactions and higher concurrency like updates or point lookups. A cross-join of this type consumes a lot of RAM, CPU threads and also starves the web requests, which can cause client timeouts.
```