/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  swcMinify: true,
  // Ensure production builds work correctly
  output: 'standalone',
  // Configure image domains if needed
  images: {
    domains: ['localhost'],
  },
}

module.exports = nextConfig
