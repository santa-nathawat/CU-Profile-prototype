import path from 'node:path';
import type { NextConfig } from 'next';

export default {
  agentRules: false,
  output: 'standalone',
  outputFileTracingRoot: path.join(__dirname, '../..'),
} satisfies NextConfig;
