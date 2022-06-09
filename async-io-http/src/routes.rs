use crate::router::Router;

// radix tree example:
// def won't compile initial example
//                '/'
//              /      \
//            foo      bar
//                     / \
//                  baz   abc
pub fn configure(router: &mut Router) {
    router.insert(Method::GET, "/", index);
    router.insert(Method::GET, "/foo", foo);
    router.insert(Method::GET, "/bar/baz", baz);
    router.insert(Method::GET, "/bar/abc", baz);
}
