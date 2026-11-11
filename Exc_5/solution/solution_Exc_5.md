# Exercise 3 - Messaging Platforms: Fundamentals
**Done by:** Deyan Dobrev - S2510929006


## Part 2 - Line Guardian: predictive maintenance
### Stage A — Today's system

Firstly I start the **baseline_detector.py** and after this I start only 20 machines from the whole plant(**plant.py --sink postgres --machines 20**) in two separate terminals.

The machines produce **22780 readings** for ~90s and all of them are recorded in the PostgreSQL database.

**Command:**
```bash
docker exec -it postgres psql -U postgres -d factory \
  -c "SELECT count(*) FROM readings WHERE run = (SELECT max(run) FROM readings);"
```
**Output:**
```
 count
-------
 22780
```

However, the detector only read **19055 readings** from all of them and **3715 readings** are missing.

**Command:**
```bash
python3 baseline_detector.py
```
**Output:**
```
(...)
^C[baseline] stopped: read 19,055 readings, last id 22,780, 3,715 readings missing, 385 anomalies
```

After looking at the `PostgresSink` class in `plant.py`, I found out why it happens. Everything comes from how the baseline detector queries the data(`SELECT * FROM readings WHERE id > last_id ORDER BY id ASC;`) and how the readings transactions are wroking.

Looking at this code here:
```python
with conn.transaction(), conn.cursor() as cur:
    cur.executemany(
        "INSERT INTO readings (run, machine_id, seq, ts, vibration temperature, current) "
        "VALUES (%(run)s, %(machine_id)s, %(seq)s, %(ts)s, %(vibration)s, %(temperature)s, %(current)s)", 
        batch)
    time.sleep(rng.uniform(0, 0.12))
```
we see, that there is a artificial delay, which slows the transaction process of the current batcch of readings, while the next one is commited instantly. 

The SQL query is looking for the highest ID and when the next batch of readings is commited before the current one, the IDs of the current one are not queried, because the last ID of the next batch of readings is higher than of any of the reading IDs in the current batch.

An example of a more detailed explanation is:

-  **Gateway A** inserts the readings batch. PostgreSQL assigns the IDs to them - from 100 to 114. But before they are being commited, **gateway A** enters the artificial delay: `time.sleep(rng.uniform(0, 0.12))`.
- While **gateway A** is delayed, **gateway B** inserts its batch of readings, PostgreSQL assigns them the IDs from 115 to 129 and then everything gets commited.
- The baseline detector polls the available data, which is only from **gateway B**(*gateway A* is still being delayed) and assigns the `last_id` to 129.
- **Gateway A** is commited now - the readings are written into the database, but the detector cannot detect the newly commited data, because the IDs of **gateway A**(100-114) are lower than the IDs of **gateway B**(115-129). The detector only looks for `id > last_id`, which skips the readings from **gateway A**.

<br><br>
After that I ran the **baseline_detector.py** again, but this time withe the whole plant(**plant.py --sink postgres**). These were the results:

**Plant:**
```
run 1794397771: 60 machines x 10 Hz = 600 readings/s -> postgres (Ctrl+C to stop)
  t=    5s      3,060 readings sent (612/s)
  t=   10s      6,060 readings sent (606/s)
  t=   15s      9,060 readings sent (604/s)
  t=   20s     12,060 readings sent (603/s)
  t=   25s     15,060 readings sent (602/s)
  t=   30s     18,060 readings sent (602/s)
  t=   35s     21,060 readings sent (602/s)
  t=   40s  fault starts on M-029 (repaired in 45 s)
  t=   40s     24,060 readings sent (601/s)  faulty: M-029
  t=   45s     27,060 readings sent (601/s)  faulty: M-029
  t=   50s     30,060 readings sent (601/s)  faulty: M-029
  t=   55s     33,060 readings sent (601/s)  faulty: M-029
  t=   60s     36,060 readings sent (601/s)  faulty: M-029
  t=   65s     39,060 readings sent (601/s)  faulty: M-029
^Crun 1794397771: sent 39,600 readings in 66 s
```

**Detector:**
```
[baseline] polling table readings for id > 22780 (Ctrl+C to stop)
[baseline] read        0  backlog       0 rows  delay   0.0 s  missing      0  anomalies 0
[baseline] read       15  backlog       0 rows  delay   0.1 s  missing      0  anomalies 0
[baseline] read    1,620  backlog   1,742 rows  delay   3.0 s  missing     30  anomalies 0
[baseline] read    3,347  backlog   3,465 rows  delay   5.9 s  missing     73  anomalies 0
[baseline] read    5,347  backlog   5,605 rows  delay   9.4 s  missing     88  anomalies 0
[baseline] read    7,347  backlog   7,880 rows  delay  13.2 s  missing     88  anomalies 0
[baseline] read    9,347  backlog   9,960 rows  delay  16.7 s  missing     88  anomalies 0
[baseline] read   11,347  backlog  12,025 rows  delay  20.1 s  missing     88  anomalies 0
[baseline] read   13,347  backlog  14,120 rows  delay  23.6 s  missing     88  anomalies 0
[baseline] read   15,347  backlog  16,170 rows  delay  27.1 s  missing     88  anomalies 0
[baseline] read   17,347  backlog  18,265 rows  delay  30.4 s  missing     88  anomalies 0
[baseline] read   19,347  backlog  20,120 rows  delay  34.0 s  missing     88  anomalies 0
^C[baseline] stopped: read 19,347 readings, last id 42,632, 88 readings missing, 0 anomalies
```

The thing is, that the firts fault starts at 40s again, but this time the detector didn't detected this - even 25s after the fault started. What actually happened?

- **delay** - The results shows, that the delay is increasing incredibly fast. Before I stopped the detector, there was 34s delay. This happens, because the detector struggles to process everything - it gets more data, than it can actually process. We can also see this in the **backlog**. This fails our idea of predictive maintenance.

- **backlog** - It also increases extremely fast, which is not the behavior, that we want. Again the reason behind this, is that the unread rows pile up in the database faster than the detector can clear them.

- **missing** - This is caused, becasue of the race condition, that I explained above. Multiple parallel gateways are trying to insert batches, but they are delayed before being able to commit. Rows with higher IDs finish before rows with lower IDs. The detector polls the higher IDs first and skips the delayed lower ID rows. Then the database write pattern stabilize into a steady rhythm and the number of skipped rows capps out at 88.

`Stage A` shows, that requirements **R1**(Scale), **R2**(Complete and ordered) and **R3**(Fresh) failed to be satisfied.

<br><br>

### Stage B — Decide

1. **Option 1** - This options keeps PostgreSQL, but polls it better. The database and the message queue are kept, but they are optimized - splitting the workload across multiple pollers(e.g. 1st poller handles machines 1 to 20, second one handles machines 21 to 40 and so on...), adding better indexes or partition the table(e.g. by machine ID). But some requirements still won't be satisfied.

    - **R1**(Scale) - This requirement will fail, because of the nature of PostgresSQL - it is a relational database used for transactional consistency and relational queries. Stage A shows, that it struggles with 600 readings/s, now we can imagine 100000 readings/s. This is not very optimal, because it can cause dead tuples, bloat of the database and crashing or freezing of the system.

    - **R2**(Complete and ordered) - This requirement is at risk, because even if multiple pollers are used, the relational database and queries like `id > last_id` are still being used. This can stil cause missing data anomalies or race conditions.

    - **R3**(Fresh) - This requirement will fail, because the query performance gets worse as the table grows to million or even billion of rows. Imagine polling such table - there would be query latencies, which cannot ensure, that anomalies can be caught on time.

    - **R4**(Independent consumers) - This requirement would fail, because this means, that we need additional poller(e.g. a live web dashboard would need to poll the database). Every new consumer is slowing the process more and more. 

    The tipping point for this option is around 200 readings/s. Everything after this thereshold causes the database to start fail under the heavy load  - constantly managing table locks and scanning growing indexes.

<br><br>

2. **Option 2** - This options uses **Apache Kafka** as an event-streaming platform instead of PostgreSQL. Every gateway will now write every reading into the Kafka topic. The detector will play the role of a consumer and PostgreSQL will be used only for static data. The requirements for this option would be satisfied.

    - **R1**(Scale) - This requirement would be satisfied. Kafka can handle these 100000 readings/s, because it uses sequential disk writes and append only logs. These event-streaming platforms and specifically Kafka are build for high-throughput and massive data streams.

    - **R2**(Complete and ordered) - This requirement would also be satisfied. The logic behind the Kafka's partitioning allows for strict chronological order of the readings. When using `machine_id`, all the readings for a specific machine will be stored on the same partition. Also thanks to `offsets` and `consumer groups`, no reading would be skipped.

    - **R3**(Fresh) - This requirement would also be satisfied. Kafka doesn't use SQL quieries, which means, that the processing delays would be minimal(under 5s). This is thanks to the way in which Kafka streams the data directly to consumers.

    - **R4**(Independent consumers) - And of course this requirement would succeed. The whole architecture behind Kafka is build around independent consumers. Adding new consumers doesn't change the process load. This means, that the detector, a live dashboard and the training data can all consume the readings at the same time without any problems(unlike Option 1).

    The tipping point for this option is higher than the tipping point for option 1. If the AI model needs 3ms per reading, this means, that one detector can process ~333 readings/s(1000ms/3ms per reading). So for 600 readings/s, we need 2 detectors and for 100000 readings/s we need ~300 detectors. This is not a problem for Kafka, because it can scale out horizontally by splitting its topics(partitioning).

<br><br>

### Stage C — Challenge: Build the core

### 1. Gateways:

My implementation for `KafkaSink` is:
```python
class KafkaSink:
    """Your setup: the gateways send every reading to Kafka.

    TODO (Part 2, Stage C1). clients/producer.py of Part 1 shows
    how to produce. Decide, and justify in your decision record: which topic
    (created how?), which key, which producer settings?
    """

    def __init__(self, args):
        from confluent_kafka import Producer
        self.producer = Producer({
            "bootstrap.servers": args.bootstrap,
            "compression.type": "lz4",
            "acks": "all"
        })

    def send(self, reading):
        data_bytes = json.dumps(reading).encode("utf-8")
        id_bytes = reading["machine_id"].encode("utf-8")

        self.producer.produce(
            topic="readings",
            key=id_bytes,
            value=data_bytes
        )

    def tick(self):
        """Called once per tick, after all readings of that tick were sent."""
        self.producer.poll(0)

    def close(self):
        self.producer.flush(10)
```

The things I added into the `__init__` function:

- `"compression.type": "lz4"` - This is a compression algorithm, which helps with the high-throughput streaming when sending the JSON strings raw. It is the industry standart for optimizing speed.

- `"acks": "all"` - This parameter controls how strictly the Kafka broker confirms that a message has been saved, before instructing the producer the send the new message. When setting `acks` to `all`, this guarantees maximum durability and ensure that no message is dropped in the process.

- ``` 
    def send(self, reading):
        data_bytes = json.dumps(reading).encode("utf-8")
        id_bytes = reading["machine_id"].encode("utf-8")

        self.producer.produce(
            topic="readings",
            key=id_bytes,
            value=data_bytes
        )
    ```
    This funtion takes the raw Python data, formats it into bytes(because Kafka doesn't understand Python dictionaries), assings a routing key and send it asynchronously to the Kafka broker. 

### 2. Detector:

Added:

```python
ap.add_argument("--in-topic", default="readings", help="TODO: the topic your gateways write to")
ap.add_argument("--group", default="detectors", help="TODO: the consumer group of this detector")
```
The `default` for the first argument is `reading`, because this is the name of the topic. The `default` for the second argument is `detectors`, because this ensures, that Kafka will automatically split the partitions between them if I run multiple detectors. They would share the workload without the duplicating work.

<br>

```python
"auto.offset.reset": "latest"
```
Setting this parameter to `latest` ensures, that the consumer reads only the brand new messages arriving from this moment forward, while the `earliest` option instructs the consumer to read from the very beginning of the topic log, including all the historical data.

<br>

```python
producer = Producer({
    "bootstrap.servers": args.bootstrap,
    "compression.type": "lz4",
    "acks": "all"
})
```
I already explained that above.

<br>

```python
anomaly_key = reading["machine_id"].encode("utf-8")
            
producer.produce(
    args.out_topic, 
    key=anomaly_key,
    value=json.dumps(event)
)
```
For the anomaly key again I use `reading["machine_id"].encode("utf-8")`. This ensures, that all anomaly events for a specific machine land in the same partition of the `anomalies` topic.

After that I start 2 instances of the `detector_service.py`.

### 3. After ~3 mins:

**Scoreboard result:**
```
Run 1794406614: started 15:16:54 (3.0 min ago), 60 machines x 10 Hz, 3 faults so far

Detection  (topic anomalies: 340 anomaly events, models ['v1'])
   #  machine   start  detected   delay
   1  M-029       40s       yes   8.0 s
   2  M-034      100s        NO       -
   3  M-015      160s  (just started, not scored yet)

  faults detected: 1/2, mean delay 8.0 s; false alarms (episodes on healthy machines): 0
  latency reading -> anomaly event: p50 64.12 s, p95 79.00 s, max 80.74 s  (replayed data? latency only meaningful for live processing)
  duplicate anomaly events (same anomaly_id): 0

Detector instances  (model telemetry, this run)
  det-1          pid 844020  v1  readings    48,128  missing       0  out-of-order      0  machines   60  silent for 18 s

Requirements (measurable part)
  [PASS] R2 complete and ordered: 0 readings missing, 0 out of order
  [FAIL] R3 fresh: p95 latency < 5 s
  [FAIL] detection: all faults found, at most 2 false alarms
```

### Fix:
- Run the topic with 2 partitions:
```bash
docker exec -it kafka kafka-topics.sh --bootstrap-server localhost:9092 --create --topic readings --partitions 2 --replication-factor 1
```

- Run two detector instances:
```bash
python3 detector_service.py --name det-1 --group detectors
```
```bash
python3 detector_service.py --name det-2 --group detectors
```

### Fix result:
```
Run 1794407316: started 15:28:36 (4.1 min ago), 60 machines x 10 Hz, 3 faults so far

Detection  (topic anomalies: 1,266 anomaly events, models ['v1'])
   #  machine   start  detected   delay
   1  M-029       40s       yes   7.9 s
   2  M-005      100s       yes   7.3 s
   3  M-059      160s       yes   5.4 s

  faults detected: 3/3, mean delay 6.9 s; false alarms (episodes on healthy machines): 0
  latency reading -> anomaly event: p50 1.73 s, p95 3.85 s, max 4.10 s
  duplicate anomaly events (same anomaly_id): 0

Detector instances  (model telemetry, this run)
  det-1          pid 885239  v1  readings    64,112  missing       0  out-of-order      0  machines   60  silent for 29 s
  det-2          pid 885294  v1  readings    64,664  missing       0  out-of-order      0  machines   30  silent for 26 s

Requirements (measurable part)
  [PASS] R2 complete and ordered: 0 readings missing, 0 out of order
  [PASS] R3 fresh: p95 latency < 5 s
  [PASS] detection: all faults found, at most 2 false alarms
```