pub mod router;
use std::collections::HashMap;

#[derive(PartialEq, Eq, Hash)]
pub enum Method {
    GET,
    POST,
    PUT,
    DELETE
}

pub struct Router {

    routes: HashMap<Method, ...>,
}

// router should implement
// radix trees: a data structure that represents a space-optimized prefix tree (trie)
