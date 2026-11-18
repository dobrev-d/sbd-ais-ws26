# Exercise 6 - Card Sentinel: real-time fraud detection with CDC
**Done by:** Deyan Dobrev - S2510929006


## Part 1 - The decision (in the session)
If I had to choose an option for this scenario, I would choose option 4 - the `Log-based CDC`. The reason behind my choice is, that only option 4 satisfies **R1(System of record untouched)**.
- Options 2 and 3 - `Dual  write` and `Outbox` modify the certified payment path, which is strictly forbidden from **R1** and option 1 - the `Polling` adds query load(by constantly querying the database) and also misses payments that commit out of id order.

However, my choice has a trade-off: the `Log-based CDC` method uses replication slot, which means, that it keeps all of the WALs(write-ahead logs), which haven't been read by the connector yet. Now if we imagine, that for some reason the connector stops reading these logs, they are going to accumulate in the disk storage, which may cause other issues like stopping the database from operating.

So, in order to detect such issues in production, before an outage occurs, the following things should constantly be monitored:
- the disk usage of the system
- the replication slot lag - the difference between the oldest and the newest WAL. When this lag crosses a certain thereshold, the team is alerted, that there is a problem with the connector.
- connector status - checking if the connector is running properly

Now if my team owned the code and when I assume, that I could change the code(because of an ownership), I would choose option 3 - the `Outbox` method. This method satisfies all the requirements and sometimes it even beats the performance of the `Log-based CDC`: Instead of having a replication slot, which can cause issues, when the connector isn't working, the `Outbox` pattern writes all transaction events in a separate table in parallel. When the relay polls the processed rows, they automatically get deleted. If the relay service stops, the outbox table continue to grow, but this doesn't affect the database performance in the way, that the replication slot affects it.

## Part 2 - The pipeline in three runs (in the session)
### Run 1:

**`Prediction:`** My prediction for the first run is, that **R4** won't pass, because if only one instance works, it can take at most around 500 payments/s, but we generate 1000 payments/s, which mean, that because of the lag, alerts will take more than 5s to be announced.

**`Velocity lines:`**
```
Run 1795013175: started 15:46:15, 130,281 transactions in 130 s (1,001/s), 13 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   1        1        42
  velocity-agent           fraud.public.transactions                   1        3    69,728

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1         455833 anomaly         129,088       72,545             0      0.4       4s ago
  vel-1         455834 velocity         50,459       39,542             0     70.1       0s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          2/6  (33%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  55.7 s / 68.6 s   FAIL  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          6/6  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.8 s / 1.0 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```

From the result of `scoreboard.py`, it becomes clear, how only one velocity model performed in our system. There was 130,281 transactions and **vel-1** read only around 50,000 of them. Also the lag was 70 s. **R3** was partly satisfied, but **R4** wasn't fulfilled. We can also see, that we have one partition for three members of the **velocity-agent** group.

**`Why:`** The velocity instances won't work together, because by default there is only one partition, which would be assigned to only one of the velocity instances(**vel-1**). This can be seen in the Kafka UI. Usually each partition can be read only by one instance within a consumer group. When using only 1 partition, only **vel-1** uses it, while **vel-2** and **vel-3** stay idle. The thing is, one velocity instance can process 500 readings/s, but the system receives 1000 readings/s, which means, that every second a big lag is build up. So one instance cannot handle such traffic, while the other two do nothing.


### Run 2:

**`Prediction:`** Thanks to the hint in the `Run 2` section, I found out, that the key used for these events is the **table's primary key, which is the payment's id**. Now, the velocity model probably won't catch at least 90% of the card frauds, because the events won't be properly sorted across the partitions. Using the payment id as a key is logically incorrect. 

**`Velocity lines:`**
```
Run 1795016130: started 16:35:30, 140,188 transactions in 140 s (1,001/s), 15 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   6        1     4,420
  velocity-agent           fraud.public.transactions                   6        3     4,499

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1         632683 anomaly         139,289       75,177             0      0.8       4s ago
  vel-1         632684 velocity         44,222       35,691             0      0.8       4s ago
  vel-2         632685 velocity         44,688       36,020             0      0.7       4s ago
  vel-3         632686 velocity         44,317       35,783             1      0.7       4s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          0/7  (0%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  no alerts; worst lag of the velocity agents 1.1 s   PASS  (<= 5 s)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          7/7  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.9 s / 1.1 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```

We see, that **R4** has passed now, but **R3** is still partly fulfilled.

**`Why:`** When the thief makes 12 fast card payments on one card, each payment has been processed in a different transaction(payment id). When these IDs are hashed, the 12 payments get distributed through the different partitions. Now this won't allow for velocity model to detect the fraud, because it is a stateful per cardholder, which means, that it has to read the payments from the same card in order to find something suspicious. When these payments are split across all partitions and read from different instances, it becomes impossible for a fraud to be detected and for **R3** to be fulfilled.


### Run 3:

**`Prediction:`** After fixing the key issue, I think, that all the requirements would be fulfilled and there won't be issues. 

**`Velocity lines:`**
```
Run 1795016666: started 16:44:26, 135,185 transactions in 135 s (1,001/s), 14 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   6        1     4,204
  velocity-agent           fraud.public.transactions                   6        3     4,215

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1         664094 anomaly         132,583       73,346             0      0.8       4s ago
  vel-1         664095 velocity         43,045       24,133             0      0.8       4s ago
  vel-2         664096 velocity         42,404       23,863             0      0.8       4s ago
  vel-3         664097 velocity         42,636       24,086             0      0.8       4s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          6/6  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.9 s / 1.1 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          7/7  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.7 s / 1.1 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```
Now everything works fine and all the requirements are satisfied.

**`Why:`** After changing the key from the payment id to the user id, the transactions, which are made from the same user, are read by the same instance and sorted together in the partitions. This means, that one instance reads all of the user payments and can catch the fraud. 


## Final answer - Debezium chose two defaults that you had to change. What would have happened in production if nobody had checked them, and how could the team have noticed?

We already saw with runs 1 and 2 what would happen if nobody in production had checked these two Debezium defaults - our requirements for the service, that we build, won't be fulfilled - the card fraud system won't work properly, because:

1. One velocity instance cannot handle the amount of payments, that is received, which acts as a bottleneck in the system. We would probably get some results, but they won't be in real-time, because of the lag. The exercise proved, that even if we create more instances, the number of partitions is essential and should be considered, when creating such services.
2. When given the wrong key, the velocity instances cannot properly inspect the data and catch frauds. This default is also essential if we want for the service to run properly. It was set by default to the payment id, but this is logically wrong, because we want to check the payments for the same card/user. 

Of course the team would notice these mistakes, but the question is when and how? A possible method is **monitoring the lag** for the lag issue. This one is essential, because we saw, that the lag can act as a bottleneck and cause the losing of the real-time property for our service. But the key issue cannot be catched that easily. This is not a technical error, but a logical one and could be found after time. A method could be **monitoring the fraud rates** and looking for anomalies or **perform a service test** by creating artificial frauds and see if the system would catch them.