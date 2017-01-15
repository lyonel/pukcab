/*
Package meta implements functions for the manipulation of Pukcab index files ("catalog").


Caveats

The database uses a read-only, memory-mapped data file to ensure that
applications cannot corrupt the database. However, transactions mean that only one
process can access the index file at a given time, ANY other (read or write) access will
block.

Transactions should therefore be kept as short as possible.


*/
package meta
