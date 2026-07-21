/** @type {import('next').NextConfig} */
const nextConfig = {
  output: process.env.NEXT_STATIC_EXPORT === '1' ? 'export' : undefined,
  distDir:
    process.env.NEXT_STATIC_EXPORT === '1'
      ? '.next-static'
      : process.env.NODE_ENV === 'development'
        ? '.next-dev'
        : '.next',
  reactStrictMode: true,
  swcMinify: true,
}

module.exports = nextConfig
