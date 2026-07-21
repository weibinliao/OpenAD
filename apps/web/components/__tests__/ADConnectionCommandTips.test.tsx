import { fireEvent, render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import ADConnectionCommandTips, { AD_DISCOVERY_COMMANDS } from '../ADConnectionCommandTips';

describe('ADConnectionCommandTips', () => {
  const writeText = jest.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
  });

  afterEach(() => jest.clearAllMocks());

  test('shows safe discovery and connectivity commands', () => {
    render(<ADConnectionCommandTips locale="en" defaultOpen />);

    for (const item of AD_DISCOVERY_COMMANDS) {
      expect(screen.getByText(item.command)).toBeInTheDocument();
    }
    expect(screen.getByText(/389 is LDAP; 636 is LDAPS/)).toBeInTheDocument();
    expect(screen.getByText(/Passwords cannot be discovered/)).toBeInTheDocument();
  });

  test('copies the selected command and announces feedback', async () => {
    render(<ADConnectionCommandTips locale="en" defaultOpen />);

    fireEvent.click(screen.getByRole('button', { name: 'Copy hostname command' }));

    expect(writeText).toHaveBeenCalledWith('hostname');
    expect(await screen.findByText('Command copied')).toBeInTheDocument();
  });
});
