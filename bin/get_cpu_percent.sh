#!/bin/bash
read -r _ u1 n1 s1 i1 w1 q1 sq1 st1 _ < /proc/stat
sleep 0.3
read -r _ u2 n2 s2 i2 w2 q2 sq2 st2 _ < /proc/stat
idle1=$((i1+w1)); idle2=$((i2+w2))
tot1=$((u1+n1+s1+i1+w1+q1+sq1+st1)); tot2=$((u2+n2+s2+i2+w2+q2+sq2+st2))
dt=$((tot2-tot1)); di=$((idle2-idle1))
(( dt>0 )) && echo $(( 100*(dt-di)/dt )) || echo 0
