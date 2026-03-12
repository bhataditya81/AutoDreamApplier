/** @type {import('next').NextConfig} */
const nextConfig = {
  // Allow images from any hostname during development
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "**",
      },
    ],
  },
  // Proxy /api requests to the Go api-gateway during development.
  // In production this is handled by the ALB / ingress.
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
