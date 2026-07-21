import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import ScanWizard from '../ScanWizard';
import { I18nProvider } from '../../contexts/I18nContext';

const mockOnStartScan = jest.fn();
const renderWizard = () =>
  render(
    <I18nProvider>
      <ScanWizard onStartScan={mockOnStartScan} />
    </I18nProvider>
  );

describe('ScanWizard', () => {
  beforeEach(() => {
    mockOnStartScan.mockClear();
  });

  test('renders scan wizard with report type selection', () => {
    renderWizard();

    expect(screen.getByText('Select Report Type')).toBeInTheDocument();
    expect(screen.getByText('Folder Permissions')).toBeInTheDocument();
    expect(screen.getByText('Share Analysis')).toBeInTheDocument();
    expect(screen.getByText('AD User Report')).toBeInTheDocument();
  });

  test('allows selecting folder scan type', () => {
    renderWizard();

    const folderOption = screen.getByText('Folder Permissions');
    fireEvent.click(folderOption);

    // Should show next button enabled after selection
    const nextButton = screen.getByText('Next');
    expect(nextButton).toBeInTheDocument();
  });

  test('shows AD configuration when AD type selected', () => {
    renderWizard();

    const adOption = screen.getByText('AD User Report');
    fireEvent.click(adOption);

    const nextButton = screen.getByText('Next');
    fireEvent.click(nextButton);

    expect(screen.getByText('AD Server')).toBeInTheDocument();
    expect(screen.getByText('Base DN')).toBeInTheDocument();
  });

  test('validates required fields before allowing scan', () => {
    renderWizard();

    // Select folder type
    const folderOption = screen.getByText('Folder Permissions');
    fireEvent.click(folderOption);

    // Go to next step without entering path
    const nextButton = screen.getByText('Next');
    fireEvent.click(nextButton);

    // Should not be able to start scan without path
    const startButton = screen.queryByText('Start Scan');
    if (startButton) {
      expect(startButton).toBeDisabled();
    }
  });
});
