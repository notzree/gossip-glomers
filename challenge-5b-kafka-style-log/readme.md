Can't use an array to store the individual data becuase when we use CAS to syncrhonize and ensure atomaticity,
if we have key : array stored, then if the values to not line up, we don't know what the new values are until we 
re-read the key.
If we use prefix_key : offset and have each individial data point as a singular entry, We can easily see


latest_key : offset
log_key : []int 

We CAS latest_key : offset until we find an available offset, we then update it. Lets call this offset O.
At the point, with an array implementation we need to append value V to the array, but we also need to ensure
that log_key[O] == V
Which is not possible in a distributed system, as other nodes could be making writes to log_key which will fuckup
the order.


