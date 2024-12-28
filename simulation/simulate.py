import random

class Tree:
    def __init__(self, tree_path_count):
        self.tree_path_count = tree_path_count

    def get_random_paths(self, batch_size):
        if batch_size > self.tree_path_count:
            raise ValueError("Batch size cannot be greater than the number of paths in the tree")
        return [random.randint(1, self.tree_path_count) for _ in range(batch_size)]

    def get_unique_buckets_for_random_paths(self, random_paths):
        unique_buckets = set()
        for path_id in random_paths:
            leaf_id = self.tree_path_count + path_id - 1
            bucket_id = leaf_id
            while bucket_id > 0:
                unique_buckets.add(bucket_id)
                bucket_id = bucket_id >> 1
        return unique_buckets

EXPERIMETNS = 1000

class Simulator:
    def __init__(self, num_requests, batch_size, tree_path_count):
        self.num_requests = num_requests
        self.tree = Tree(tree_path_count)
        self.batch_size = batch_size

    def run_once(self):
        retrieved_buckets_count = 0
        for i in range(0, self.num_requests, self.batch_size):
            random_paths = self.tree.get_random_paths(self.batch_size)
            unique_buckets = self.tree.get_unique_buckets_for_random_paths(random_paths)
            retrieved_buckets_count += len(unique_buckets)
        return retrieved_buckets_count

    def run(self):
        total_buckets_count = 0
        for _ in range(EXPERIMETNS):
            total_buckets_count += self.run_once()
        print("Average number of buckets retrieved per request:", total_buckets_count / (self.num_requests * EXPERIMETNS))


if __name__ == '__main__':
    batch_size = 2000
    num_requests = 10000
    tree_path_count = 2**20
    sim = Simulator(num_requests, batch_size, tree_path_count)
    sim.run()