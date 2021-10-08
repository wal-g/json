# Streaming JSON

This is a library providing JSON serialization by streaming to given io.WriteCloser, without allocating memory for entire JSON 
and deserialization object from io.Reader without fetching all the data before.

### Credits

This library is strongly based on the standart library encoding/json package.
