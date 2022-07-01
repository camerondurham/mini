#include<iostream>
#include<map>
#include<string>
#include<priorityqueue>
#include<memory>

// TODO: make item generic
struct item {
  int value;
};

class PriorityExpiryCache {
  int maxItems;
  std::map<std::string,item> items;
  // TODO: what is the smart pointer type???
  std::map<unsigned int, std::smart_ptr> expiryIndexMap;

};

int main() {
  std::cout << "== PRIORITY EXPIRY CACHE TESTS ==\n";

}
