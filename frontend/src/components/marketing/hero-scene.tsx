'use client';
import { useRef, useMemo } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { Sphere, MeshDistortMaterial, Float, Stars } from '@react-three/drei';
import * as THREE from 'three';

function FloatingOrb({ position, scale, speed, color }: { position: [number,number,number]; scale: number; speed: number; color: string }) {
  const ref = useRef<THREE.Mesh>(null);
  useFrame((state) => {
    if (!ref.current) return;
    ref.current.rotation.x = state.clock.elapsedTime * speed * 0.3;
    ref.current.rotation.y = state.clock.elapsedTime * speed * 0.5;
  });
  return (
    <Float speed={speed} rotationIntensity={0.4} floatIntensity={1.5}>
      <Sphere ref={ref} args={[1, 64, 64]} position={position} scale={scale}>
        <MeshDistortMaterial
          color={color}
          attach="material"
          distort={0.45}
          speed={2}
          roughness={0}
          metalness={0.1}
          transparent
          opacity={0.75}
        />
      </Sphere>
    </Float>
  );
}

function ParticleField() {
  const count = 600;
  const positions = useMemo(() => {
    const arr = new Float32Array(count * 3);
    for (let i = 0; i < count; i++) {
      arr[i * 3] = (Math.random() - 0.5) * 20;
      arr[i * 3 + 1] = (Math.random() - 0.5) * 20;
      arr[i * 3 + 2] = (Math.random() - 0.5) * 20;
    }
    return arr;
  }, []);
  const ref = useRef<THREE.Points>(null);
  useFrame((state) => {
    if (ref.current) ref.current.rotation.y = state.clock.elapsedTime * 0.03;
  });
  return (
    <points ref={ref}>
      <bufferGeometry>
        <bufferAttribute attach="attributes-position" args={[positions, 3]} />
      </bufferGeometry>
      <pointsMaterial size={0.04} color="#818cf8" transparent opacity={0.7} sizeAttenuation />
    </points>
  );
}

export default function HeroScene() {
  return (
    <Canvas camera={{ position: [0, 0, 8], fov: 60 }} style={{ background: 'transparent' }}>
      <ambientLight intensity={0.5} />
      <pointLight position={[10, 10, 10]} intensity={1.5} color="#6366f1" />
      <pointLight position={[-10, -10, -5]} intensity={1} color="#8b5cf6" />
      <Stars radius={80} depth={50} count={1000} factor={3} saturation={0} fade speed={0.5} />
      <ParticleField />
      <FloatingOrb position={[0, 0, 0]} scale={1.8} speed={0.6} color="#6366f1" />
      <FloatingOrb position={[-3.5, 1.5, -2]} scale={0.7} speed={0.9} color="#8b5cf6" />
      <FloatingOrb position={[3.5, -1, -1]} scale={0.5} speed={1.2} color="#a78bfa" />
      <FloatingOrb position={[2, 2.5, -3]} scale={0.4} speed={0.7} color="#c4b5fd" />
    </Canvas>
  );
}
