<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
<title>Okoko</title>
</head>

<body>


<script src="<?php echo base_url();?>assets/js/bootstrap.bundle.min.js"></script>
<script src="<?php echo base_url();?>assets/js/jquery-3.6.4.min.js"></script>
<script src="<?php echo base_url();?>assets/js/popper.min.js"></script>
<link rel="stylesheet" href="<?php echo base_url();?>assets/css/bootstrap.min.css">



<nav class="navbar navbar-expand-lg bg-body-tertiary">
  <div class="container-fluid">
    
    <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbarSupportedContent" aria-controls="navbarSupportedContent" aria-expanded="false" aria-label="Toggle navigation">
      <span class="navbar-toggler-icon"></span>
    </button>
    <div class="collapse navbar-collapse" id="navbarSupportedContent">
      <ul class="navbar-nav me-auto mb-2 mb-lg-0">
        <li class="nav-item">
          <a class="nav-link active" aria-current="page" href="<?php echo site_url();?>">Info temps réel</a>
        </li>
          <li class="nav-item dropdown">
          <a class="nav-link dropdown-toggle" href="#" role="button" data-bs-toggle="dropdown" aria-expanded="false">
            Graphiques
          </a>
          <ul class="dropdown-menu">
            <li><a class="dropdown-item" href="<?php echo site_url();?>/Site/temp">Températures</a></li>
			<li><hr class="dropdown-divider"></li>
            <li><a class="dropdown-item" href="<?php echo site_url();?>/Site/conso">Consommation de pellets</a></li>
            <li><hr class="dropdown-divider"></li>
            <li><a class="dropdown-item" href="<?php echo site_url();?>/Site/modulation">Modulation de la chaudière</a></li>
			<li><hr class="dropdown-divider"></li>
            <li><a class="dropdown-item" href="<?php echo site_url();?>/Site/stock">Stock de pellets</a></li>
          </ul>
        </li>
       
	   
	   
	   
	   
      </ul>
     
    </div>
  </div>
</nav>





