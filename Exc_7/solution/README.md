# Exercise 7 - From the log to the lake
**Done by:** Deyan Dobrev - S2510929006


# Part 1

## 1. Run the example

First we run the `run_once()` function from **Section 1** in the notebook. This function reads the topic from the beginning, every time and each run does:
- **read**: reads the new records of the topic `logs`
- **write**: writes them into the lake(as a Parquet file)
- **remember**: remembers where it stopped

The first run was successful:
```
reset: lake, staging and state deleted
read_new: 2,000,000 new records
write_files: 2,000,000 records -> /home/jovyan/data/lake/logs/run-20261125-112225-3cce4d

Lake check — topic 'logs' (4 partitions)   2026-11-25 11:22:53
lake: read_parquet('/home/jovyan/data/lake/logs/**/*.parquet')

partition          topic offsets     in lake  duplicates    missing  not loaded
-------------------------------------------------------------------------------
        0           [0, 507,867)     507,867           0          0           0
        1           [0, 518,590)     518,590           0          0           0
        2           [0, 468,151)     468,151           0          0           0
        3           [0, 505,392)     505,392           0          0           0
-------------------------------------------------------------------------------
    total      2,000,000 records   2,000,000           0          0           0

files under /home/jovyan/data/lake: 4 data files, 111.5 MB, 27.86 MB per file on average
check took 0.2 s

Exactly once: PASS
{'topic': 2000000,
 'lake': 2000000,
 'dup': 0,
 'miss': 0,
 'lag': 0,
 'files': 4,
 'mb': 111.457888,
 'ok': True,
 'problems': []}
```

The second run however wasn't successful:
```
read_new: 2,000,000 new records
write_files: 2,000,000 records -> /home/jovyan/data/lake/logs/run-20261125-112253-9f83aa

Lake check — topic 'logs' (4 partitions)   2026-11-25 11:23:19
lake: read_parquet('/home/jovyan/data/lake/logs/**/*.parquet')

partition          topic offsets     in lake  duplicates    missing  not loaded
-------------------------------------------------------------------------------
        0           [0, 507,867)   1,015,734     507,867          0           0
        1           [0, 518,590)   1,037,180     518,590          0           0
        2           [0, 468,151)     936,302     468,151          0           0
        3           [0, 505,392)   1,010,784     505,392          0           0
-------------------------------------------------------------------------------
    total      2,000,000 records   4,000,000   2,000,000          0           0

files under /home/jovyan/data/lake: 8 data files, 222.9 MB, 27.86 MB per file on average
check took 0.3 s

Exactly once: FAIL — 2,000,000 duplicates
{'topic': 2000000,
 'lake': 4000000,
 'dup': 2000000,
 'miss': 0,
 'lag': 0,
 'files': 8,
 'mb': 222.915776,
 'ok': False,
 'problems': ['2,000,000 duplicates']}

```

Here the second checker fails, because the `read_new(None)` starts to read from the beginning of the topic, which means, that it reads all 2000000 records again and writes a copy of thoese records into the `LAKE`. So intead of 2000000 records, there are 4000000 records into the `LAKE`.

## 2. Choose an architecture

For an **architecture** choice, we have 3 options:
- **A - State file**: This architecture write the Parquet files first and then save the last processed offset per partition to a small state file. It is very simple, but when a crash occurs right after the writing data and before saving the state file, the next run can re-read the same offsets. So it is simple, but the trade-off is, that the lake and the state file must stay in sync, which makes this architecture vulnerable to duplicates if a crash happens.
- **B - The lake is the truth**: Instead of keeping a separate file, this architecture inspects the `LAKE` before writing so it can find the highest offset, then it writes the new batch to `STAGE` and publish it into the `LAKE` in one step. So if a crash occurs, the data is read only from the `LAKE` and the staging files stay invisible. On the next run the previous position is returned so it doesn't produce duplicates. This is the advantage, but a trade-off is, that cloud object stores do not support this architecture.
- **C - Remove duplicates when reading**: Using this architecture, the function append records. If it crashes, duplicates may occur, but on read a view(DEDUP_SQL) is used to remove duplicates/deduplicate by keeping one row per **(partition, offset)**. It is very easy to write, but duplicate files can accumulate into the lake and also these queries are costly.

For this task I choose to implement the second option: **B - The lake is the truth**, because it offers a really good functionality and has the least worst trade-offs compared to the other two architectures. An advantage, that is important for my choice is, that using this architecture, the run folder is moved from `STAGING` into the `LAKE` in a single step - this means, that the batch is in the `LAKE` and is visible to readers or it is not, simple as that.

## 3. Implement

First I call `lake_position()` to look into the `LAKE` and find the latest/highest offset. Then I use `read_new()` to get the new records up to the current end of the topic. Then I use `write_files()` in order to write the files(even incomplete ones) to the staging folder, which is invisible for the readers. At the end I call `publish()` to move the staging folder into the `LAKE`.

So the function looks like this:
```python
def run_once():
    # 1. where did the last run stop?
    start = lake_position()
    # 2. read what is new
    new = read_new(start)
    # 3. write it
    run = write_files(new, STAGING)
    # 4. remember where you stopped (if your architecture needs it)
    publish(run)
```

## 4. Check
After implementing this architecture and running the `check_all()` function, this is my result:

```
reset: lake, staging and state deleted

situation                                 in lake  duplicates  missing  not loaded    run  check
-----------------------------------------------------------------------------------------------
1 first run                             2,000,000           0        0           0  30.9s  PASS
2 new records arrive, run again         2,100,000           0        0           0   5.5s  PASS
3 nothing new, run again                2,100,000           0        0           0   0.0s  PASS
4 run stops WHILE writing: right after  2,100,000           0        0     100,000         PASS
  ... then the next run                 2,200,000           0        0           0   4.8s  PASS
5 run stops AFTER writing: right after  2,200,000           0        0     100,000         PASS
  ... then the next run                 2,300,000           0        0           0   4.7s  PASS
-----------------------------------------------------------------------------------------------
Exactly once after every run: YES
[('1 first run',
  {'topic': 2000000,
   'lake': 2000000,
   'dup': 0,
   'miss': 0,
   'lag': 0,
   'files': 4,
   'mb': 111.457888,
   'ok': True,
   'problems': []},
  30.891253232955933),
 ('2 new records arrive, run again',
  {'topic': 2100000,
   'lake': 2100000,
   'dup': 0,
   'miss': 0,
   'lag': 0,
   'files': 8,
   'mb': 112.594492,
   'ok': True,
   'problems': []},
  5.4544572830200195),
 ('3 nothing new, run again',
  {'topic': 2100000,
   'lake': 2100000,
   'dup': 0,
   'miss': 0,
   'lag': 0,
   'files': 8,
   'mb': 112.594492,
   'ok': True,
   'problems': []},
  0.04112577438354492),
 ('4 run stops WHILE writing: right after',
  {'topic': 2200000,
   'lake': 2100000,
   'dup': 0,
   'miss': 0,
   'lag': 100000,
   'files': 8,
   'mb': 112.594492,
   'ok': True,
   'problems': []},
  None),
 ('  ... then the next run',
  {'topic': 2200000,
   'lake': 2200000,
   'dup': 0,
   'miss': 0,
   'lag': 0,
   'files': 12,
   'mb': 113.751678,
   'ok': True,
   'problems': []},
  4.839391708374023),
 ('5 run stops AFTER writing: right after',
  {'topic': 2300000,
   'lake': 2200000,
   'dup': 0,
   'miss': 0,
   'lag': 100000,
   'files': 12,
   'mb': 113.751678,
   'ok': True,
   'problems': []},
  None),
 ('  ... then the next run',
  {'topic': 2300000,
   'lake': 2300000,
   'dup': 0,
   'miss': 0,
   'lag': 0,
   'files': 16,
   'mb': 114.903075,
   'ok': True,
   'problems': []},
  4.692467212677002)]
```