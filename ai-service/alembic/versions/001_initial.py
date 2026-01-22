"""Initial schema for AI microservice

Revision ID: 001_initial
Revises: 
Create Date: 2026-01-22

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa
from pgvector.sqlalchemy import Vector

# revision identifiers, used by Alembic.
revision: str = '001_initial'
down_revision: Union[str, None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Enable pgvector extension
    op.execute('CREATE EXTENSION IF NOT EXISTS vector')

    # Create ai_conversation table
    op.create_table(
        'ai_conversation',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('uid', sa.String(36), nullable=False),
        sa.Column('creator_id', sa.Integer(), nullable=False),
        sa.Column('title', sa.String(255), nullable=False, server_default=''),
        sa.Column('provider', sa.String(50), nullable=False),
        sa.Column('model', sa.String(100), nullable=False),
        sa.Column('system_prompt', sa.Text(), nullable=True),
        sa.Column('row_status', sa.String(20), nullable=False, server_default='NORMAL'),
        sa.Column('created_ts', sa.BigInteger(), nullable=False),
        sa.Column('updated_ts', sa.BigInteger(), nullable=False),
        sa.PrimaryKeyConstraint('id'),
        sa.UniqueConstraint('uid'),
    )
    op.create_index('idx_ai_conversation_creator', 'ai_conversation', ['creator_id'])
    op.create_index('idx_ai_conversation_creator_status', 'ai_conversation', ['creator_id', 'row_status'])

    # Create ai_message table
    op.create_table(
        'ai_message',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('uid', sa.String(36), nullable=False),
        sa.Column('conversation_id', sa.Integer(), nullable=False),
        sa.Column('role', sa.String(20), nullable=False),
        sa.Column('content', sa.Text(), nullable=False),
        sa.Column('token_count', sa.Integer(), nullable=False, server_default='0'),
        sa.Column('created_ts', sa.BigInteger(), nullable=False),
        sa.PrimaryKeyConstraint('id'),
        sa.UniqueConstraint('uid'),
        sa.ForeignKeyConstraint(['conversation_id'], ['ai_conversation.id'], ondelete='CASCADE'),
    )
    op.create_index('idx_ai_message_conversation', 'ai_message', ['conversation_id'])

    # Create document_embedding table for RAG
    op.create_table(
        'document_embedding',
        sa.Column('id', sa.Integer(), autoincrement=True, nullable=False),
        sa.Column('user_id', sa.Integer(), nullable=False),
        sa.Column('document_type', sa.String(20), nullable=False),  # 'memo' or 'attachment'
        sa.Column('document_uid', sa.String(36), nullable=False),
        sa.Column('chunk_index', sa.Integer(), nullable=False),
        sa.Column('chunk_text', sa.Text(), nullable=False),
        sa.Column('embedding', Vector(1536), nullable=False),  # OpenAI ada-002 dimensions
        sa.Column('created_ts', sa.BigInteger(), nullable=False),
        sa.Column('updated_ts', sa.BigInteger(), nullable=False),
        sa.PrimaryKeyConstraint('id'),
    )
    op.create_index('idx_document_embedding_user', 'document_embedding', ['user_id'])
    op.create_index('idx_document_embedding_document', 'document_embedding', ['document_type', 'document_uid'])
    
    # Create vector similarity index using HNSW (faster for queries)
    op.execute('''
        CREATE INDEX idx_document_embedding_vector 
        ON document_embedding 
        USING hnsw (embedding vector_cosine_ops)
        WITH (m = 16, ef_construction = 64)
    ''')


def downgrade() -> None:
    op.drop_table('document_embedding')
    op.drop_table('ai_message')
    op.drop_table('ai_conversation')
    op.execute('DROP EXTENSION IF EXISTS vector')
