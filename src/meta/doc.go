/*
Package meta implements functions for the manipulation of Pukcab index files ("catalog").


Caveats

The database uses a read-only, memory-mapped data file to ensure that
applications cannot corrupt the database, however, this means that keys and
values returned cannot be changed. Writing to a read-only byte slice
will cause Go to panic.

Keys and values retrieved from the database are only valid for the life of
the transaction. When used outside the transaction, these byte slices can
point to different data or can point to invalid memory which will cause a panic.


*/
package meta
