
from abc import ABC, abstractmethod


"""
ABC 是 Abstract Base Class ，抽象基类
@abstractmethod 是抽象方法装饰器，用来标记“这个方法只是定义规范，不提供完整实现，子类必须实现
"""
class MemoryBase(ABC):
    
    @abstractmethod
    def get(self, memory_id):
        """
        Retrieve a memory by ID

        Args: 
            memory_id (str): ID of the memory to retrieve.

        Return:
            dict: Retrieved memory.
        """
        pass


    @abstractmethod
    def get_all(self):
        """
        List all memories

        Return:
            list: List of all memories
        """

        pass

    @abstractmethod
    def update(self, memory_id, data):
        """
        Update a memory by ID.

        Args:
            memory_id (str): ID of the memory to update.
            date (str): New content to update the memory with.

        Return: 
            dict: Success message indicating the memory was updated.
        """
        pass

    @abstractmethod
    def delete(self, memory_id):
        """
        Delete a memory by a ID.

        Args:
            memory_id (str): ID of the memory to delete
        """
        pass

    @abstractmethod
    def history(self, memory_id):
        """
        Get the history of change for a memory by ID

        Args:
            memory_id (str): ID of the memory to get history for.

        Returns:
            list: list of changes for the memory.
        """
        pass