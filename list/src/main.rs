use std::mem;

#[derive(Debug)]
struct List<T> {
    head: Option<Box<Node<T>>>,
} 

#[derive(Debug)]
struct Node<T> {
    elem: T,
    next: Option<Box<Node<T>>>,
}


impl<T> List<T> {
    pub fn new() -> Self {
        List { head: None }
    }

    pub fn push(&mut self, elem: T) {
        let new_node = Box::new(Node {
            elem: elem,
            next: mem::replace(&mut self.head, None),
        });

        self.head = Some(new_node);
    }

    pub fn pop(&mut self) -> Option<T> {
        match mem::replace(&mut self.head, None) {
            None => None,
            Some(node) => {
                self.head = node.next;
                Some(node.elem)
            }
        }
    }
    
    pub fn contains(&self, item: T) -> bool {
        match &self.head {
            None => return false,
            Some(node) => {
                let cur = node;
                return false;
            }
        }
    }
}

fn main() {
  let mut l: List<i32> = List::new();
  l.push(42);
  l.push(43);
  l.push(44);
  println!("{l:?}");

  l.contains(33);
}

