use std::net::TcpListener;
use std::io::Result;

fn main() -> Result<()> {
    let listener = TcpListener::bind("127.0.0.1:7000")?;

    for client listener.incoming() {
        // block until reading client completes
        // ... read from client parse output
    }
}
