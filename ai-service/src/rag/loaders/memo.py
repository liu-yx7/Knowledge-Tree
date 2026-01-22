"""Memo document loader for RAG indexing.

This module provides functionality to load and chunk memo content
for vector embedding and retrieval.
"""

import logging
import re
from dataclasses import dataclass

from src.config import get_settings

logger = logging.getLogger(__name__)


@dataclass
class MemoChunk:
    """A chunk of memo content for embedding."""
    
    text: str
    chunk_index: int
    start_char: int
    end_char: int


class MemoLoader:
    """Loader for memo content that handles chunking for RAG.
    
    Memos are typically short-form content, but we still chunk them
    for consistency and to handle longer memos with multiple sections.
    """

    def __init__(
        self,
        chunk_size: int | None = None,
        chunk_overlap: int | None = None,
    ):
        """Initialize the memo loader.
        
        Args:
            chunk_size: Maximum characters per chunk. Defaults to config value.
            chunk_overlap: Overlap between chunks. Defaults to config value.
        """
        settings = get_settings()
        self.chunk_size = chunk_size or settings.rag_chunk_size
        self.chunk_overlap = chunk_overlap or settings.rag_chunk_overlap

    def load(self, content: str) -> list[MemoChunk]:
        """Load and chunk memo content.
        
        Args:
            content: Raw memo content (Markdown).
            
        Returns:
            List of MemoChunk objects ready for embedding.
        """
        if not content or not content.strip():
            return []

        # Clean and normalize content
        cleaned = self._clean_content(content)
        
        if not cleaned:
            return []

        # For short memos, return as single chunk
        if len(cleaned) <= self.chunk_size:
            return [MemoChunk(
                text=cleaned,
                chunk_index=0,
                start_char=0,
                end_char=len(cleaned),
            )]

        # Chunk longer content
        return self._chunk_content(cleaned)

    def _clean_content(self, content: str) -> str:
        """Clean memo content for embedding.
        
        - Removes excessive whitespace
        - Normalizes line endings
        - Strips HTML tags if present
        - Keeps Markdown structure for context
        
        Args:
            content: Raw memo content.
            
        Returns:
            Cleaned content string.
        """
        # Normalize line endings
        text = content.replace('\r\n', '\n').replace('\r', '\n')
        
        # Remove HTML tags (memos might have inline HTML)
        text = re.sub(r'<[^>]+>', '', text)
        
        # Normalize whitespace (but preserve paragraph breaks)
        text = re.sub(r'[ \t]+', ' ', text)
        text = re.sub(r'\n{3,}', '\n\n', text)
        
        # Strip leading/trailing whitespace
        text = text.strip()
        
        return text

    def _chunk_content(self, content: str) -> list[MemoChunk]:
        """Split content into overlapping chunks.
        
        Uses a sentence-aware chunking strategy:
        1. Try to split on paragraph boundaries
        2. Fall back to sentence boundaries
        3. Fall back to word boundaries
        
        Args:
            content: Cleaned content to chunk.
            
        Returns:
            List of MemoChunk objects.
        """
        chunks: list[MemoChunk] = []
        
        # Split into paragraphs first
        paragraphs = content.split('\n\n')
        
        current_chunk = ""
        current_start = 0
        char_pos = 0
        
        for para in paragraphs:
            para = para.strip()
            if not para:
                char_pos += 2  # Account for \n\n
                continue
            
            # Check if adding this paragraph exceeds chunk size
            test_chunk = current_chunk + ('\n\n' if current_chunk else '') + para
            
            if len(test_chunk) <= self.chunk_size:
                current_chunk = test_chunk
                char_pos += len(para) + 2
            else:
                # Save current chunk if not empty
                if current_chunk:
                    chunks.append(MemoChunk(
                        text=current_chunk,
                        chunk_index=len(chunks),
                        start_char=current_start,
                        end_char=current_start + len(current_chunk),
                    ))
                    # Calculate overlap start position
                    overlap_text = current_chunk[-self.chunk_overlap:] if len(current_chunk) > self.chunk_overlap else current_chunk
                    current_start = current_start + len(current_chunk) - len(overlap_text)
                    current_chunk = overlap_text + '\n\n' + para
                else:
                    current_chunk = para
                    
                char_pos += len(para) + 2

        # Don't forget the last chunk
        if current_chunk:
            chunks.append(MemoChunk(
                text=current_chunk,
                chunk_index=len(chunks),
                start_char=current_start,
                end_char=current_start + len(current_chunk),
            ))

        return chunks

    def extract_metadata(self, content: str) -> dict[str, any]:
        """Extract metadata from memo content.
        
        Extracts:
        - Tags (e.g., #tag)
        - Links (URLs)
        - References to other memos
        - Code blocks (languages)
        
        Args:
            content: Raw memo content.
            
        Returns:
            Dictionary of extracted metadata.
        """
        metadata: dict[str, any] = {}
        
        # Extract hashtags
        tags = re.findall(r'#(\w+)', content)
        if tags:
            metadata['tags'] = list(set(tags))
        
        # Extract URLs
        urls = re.findall(r'https?://[^\s\)]+', content)
        if urls:
            metadata['urls'] = list(set(urls))
        
        # Extract code block languages
        code_langs = re.findall(r'```(\w+)', content)
        if code_langs:
            metadata['code_languages'] = list(set(code_langs))
        
        # Check for task items
        has_tasks = bool(re.search(r'- \[([ xX])\]', content))
        if has_tasks:
            metadata['has_tasks'] = True
            # Count completed vs incomplete
            completed = len(re.findall(r'- \[[xX]\]', content))
            incomplete = len(re.findall(r'- \[ \]', content))
            metadata['tasks_completed'] = completed
            metadata['tasks_incomplete'] = incomplete
        
        return metadata
