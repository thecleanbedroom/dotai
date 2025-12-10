# WordPress Project Context

## Project Overview

This is a WordPress multisite project with custom plugins and themes. The project follows WordPress coding standards and uses Lando for local development.

## Development Environment

- **Local Development**: Lando (Docker-based)
- **PHP Version**: 8.1+
- **Database**: MySQL
- **Web Server**: Nginx

## Coding Standards

- **PHP**: PSR-12 coding standards (via PHPCS)
- **JavaScript**: WordPress JavaScript coding standards
- **CSS**: WordPress CSS coding standards
- **Testing**: PHPUnit for unit testing /tests at the root and in each plugin folder ./tests

## Key Directories

- `web/wp-content/plugins/wordpress-extras/` - Custom plugin with main functionality

## Testing

- Run PHPCS before committing: `./lando-phpcs`
- Use PHPCBF for auto-fixes: `./lando-phpcbf`
- Use PHPUNIT for unit testing: `./lando-phpunit`

## Deployment

Uses Deployer for automated deployments (see deploy.php)
