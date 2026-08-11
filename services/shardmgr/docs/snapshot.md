# state change notes

 |assignState | minion | currentSnapshot | futureSnapshot|
 |   no     |   
 |   adding|
 |  ready|
|  dropping|
| dropped|

state1: ss=no , currentSnapshot=no, futureSnapshot=no,

1 -> accept (assign) -> 2

state2: ss=no, currentSnapshot=no, future=yes

2 -> minion add_shard -> 3

state3: ss=CAS_Adding, currentSnapshot=no, future=yes

3 -> minion add confirmed succ -> 4

state4: ss=CAS_Ready, currentSnapshot=yes, future=yes

4 -> accept (unassign) -> 5

state5: ss=CAS_Ready, currentSnapshot=yes, future=no

5 -> minion drop_shard -> 6

state6: ss=CAS_Dropping, currentSnapshot=yes, future=no

6 -> minion confirm drop succ -> 7

state7: ss=no, currentSnapshot=no, future=no

