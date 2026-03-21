import React from 'react';
import { render, screen } from '@testing-library/react';
import { Card, CardHeader, CardContent, CardTitle, CardDescription, CardFooter } from '@/components/ui/card';

describe('Card', () => {
  it('renders children', () => {
    render(<Card>Card content</Card>);
    expect(screen.getByText('Card content')).toBeInTheDocument();
  });

  it('has rounded styling', () => {
    const { container } = render(<Card>Content</Card>);
    expect(container.firstChild as HTMLElement).toHaveClass('rounded-xl');
  });
});

describe('CardHeader', () => {
  it('renders children', () => {
    render(<CardHeader>Header content</CardHeader>);
    expect(screen.getByText('Header content')).toBeInTheDocument();
  });
});

describe('CardTitle', () => {
  it('renders as an h3', () => {
    render(<CardTitle>My Title</CardTitle>);
    const heading = screen.getByRole('heading', { level: 3 });
    expect(heading).toHaveTextContent('My Title');
  });
});

describe('CardDescription', () => {
  it('renders description text', () => {
    render(<CardDescription>Some description</CardDescription>);
    expect(screen.getByText('Some description')).toBeInTheDocument();
  });
});

describe('CardContent', () => {
  it('renders children', () => {
    render(<CardContent>Body text</CardContent>);
    expect(screen.getByText('Body text')).toBeInTheDocument();
  });
});

describe('CardFooter', () => {
  it('renders children', () => {
    render(<CardFooter>Footer text</CardFooter>);
    expect(screen.getByText('Footer text')).toBeInTheDocument();
  });
});

describe('Card composition', () => {
  it('renders a full card with all subcomponents', () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>Title</CardTitle>
          <CardDescription>Description</CardDescription>
        </CardHeader>
        <CardContent>Body</CardContent>
        <CardFooter>Footer</CardFooter>
      </Card>
    );
    expect(screen.getByRole('heading', { name: 'Title' })).toBeInTheDocument();
    expect(screen.getByText('Description')).toBeInTheDocument();
    expect(screen.getByText('Body')).toBeInTheDocument();
    expect(screen.getByText('Footer')).toBeInTheDocument();
  });
});
